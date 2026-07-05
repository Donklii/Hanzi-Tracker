package progresso

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Vocab struct {
	Id         int
	Hanzi      string
	Pinyin     string
	Significado string
	Status     string // "estudo", "aprendido"
	DataAdd    time.Time
	TipoHanzi  string `json:"tipoHanzi"`
}

var db *sql.DB

func InitDB() error {
	appData, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dbPath := filepath.Join(appData, "HanziTracker", "progresso.db")

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	query := `
	CREATE TABLE IF NOT EXISTS vocabulario (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hanzi TEXT UNIQUE,
		pinyin TEXT,
		significado TEXT,
		status TEXT,
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS session_images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		image_base64 TEXT
	);

	CREATE TABLE IF NOT EXISTS traducoes_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		linha_original TEXT UNIQUE NOT NULL,
		traducao TEXT NOT NULL,
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS tts_audio_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pinyin TEXT NOT NULL,
		motor TEXT NOT NULL,
		audio BLOB NOT NULL,
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(pinyin, motor)
	);
	`
	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	// Migra o cache de TTS do schema antigo (chave por hanzi) para o novo (chave por pinyin).
	if err := migrarCacheTtsParaPinyin(); err != nil {
		return err
	}

	// Limpeza de legado: versões antigas gravavam as imagens de sessão nesta tabela (hoje ficam em
	// memória — ver SalvarImagemSessao); esvaziá-la recupera o espaço de bancos antigos.
	_, err = db.Exec("DELETE FROM session_images")
	return err
}

// migrarCacheTtsParaPinyin recria a tabela tts_audio_cache quando ela ainda está no schema antigo
// (chave por `hanzi`). A chave passou a ser o PINYIN para que hanzis homófonos compartilhem um único
// áudio (ver App.traduzirHanziParaChaveTts). Como o cache é descartável, a migração simplesmente
// dropa e recria — os áudios são re-sintetizados sob demanda. Idempotente: no schema novo, não faz nada.
func migrarCacheTtsParaPinyin() error {
	rows, err := db.Query("PRAGMA table_info(tts_audio_cache)")
	if err != nil {
		return err
	}
	defer rows.Close()

	temColunaHanzi := false
	for rows.Next() {
		var cid, notnull, pk int
		var nome, tipo string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &nome, &tipo, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if nome == "hanzi" {
			temColunaHanzi = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Guard clause: já está no schema novo (chave por pinyin) — nada a migrar.
	if !temColunaHanzi {
		return nil
	}

	_, err = db.Exec(`
		DROP TABLE tts_audio_cache;
		CREATE TABLE tts_audio_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pinyin TEXT NOT NULL,
			motor TEXT NOT NULL,
			audio BLOB NOT NULL,
			data_add DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(pinyin, motor)
		);
	`)
	return err
}

func AddOuUpdateVocab(hanzi, pinyin, significado, status string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	query := `
	INSERT INTO vocabulario (hanzi, pinyin, significado, status) 
	VALUES (?, ?, ?, ?)
	ON CONFLICT(hanzi) DO UPDATE SET 
		status=excluded.status,
		pinyin=excluded.pinyin,
		significado=excluded.significado;
	`
	_, err := db.Exec(query, hanzi, pinyin, significado, status)
	return err
}

// RegistrarVisto auto-salva uma palavra como 'visto' se ela ainda não existir
func RegistrarVisto(hanzi, pinyin, significado string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	vistosSessaoMu.Lock()
	if vistosSessao[hanzi] {
		vistosSessaoMu.Unlock()
		return nil
	}
	vistosSessaoMu.Unlock()

	// INSERT OR IGNORE won't update the status if the word is already 'estudo' or 'aprendido'
	query := `
	INSERT OR IGNORE INTO vocabulario (hanzi, pinyin, significado, status) 
	VALUES (?, ?, ?, 'visto')
	`
	_, err := db.Exec(query, hanzi, pinyin, significado)
	
	if err == nil {
		vistosSessaoMu.Lock()
		vistosSessao[hanzi] = true
		vistosSessaoMu.Unlock()
	}
	
	return err
}

// LimparVocabulario apaga todas as palavras do banco (zera o progresso). Como o arquivo .db
// fica aberto pelo SQLite, esvaziamos via DELETE em vez de remover o arquivo.
func LimparVocabulario() error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if _, err := db.Exec("DELETE FROM vocabulario"); err != nil {
		return err
	}
	// Recupera o espaço em disco liberado pelos registros apagados.
	_, _ = db.Exec("VACUUM")

	vistosSessaoMu.Lock()
	vistosSessao = map[string]bool{}
	vistosSessaoMu.Unlock()

	return nil
}

func RemoveVocab(hanzi string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	_, err := db.Exec("DELETE FROM vocabulario WHERE hanzi = ?", hanzi)
	if err == nil {
		vistosSessaoMu.Lock()
		delete(vistosSessao, hanzi)
		vistosSessaoMu.Unlock()
	}
	return err
}

func GetAllVocab() ([]Vocab, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query("SELECT id, hanzi, pinyin, significado, status, data_add FROM vocabulario ORDER BY data_add DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []Vocab
	for rows.Next() {
		var v Vocab
		var d string
		if err := rows.Scan(&v.Id, &v.Hanzi, &v.Pinyin, &v.Significado, &v.Status, &d); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339, d)
		v.DataAdd = t
		lista = append(lista, v)
	}
	return lista, nil
}

// ----- Imagens de sessão (crops dos cards) -----
// Efêmeras por definição (zeradas a cada sessão), então vivem em MEMÓRIA: gravá-las no SQLite
// gerava um INSERT + fsync por palavra a cada scan sem nenhum benefício de persistência. O teto
// descarta as mais antigas para a RAM não crescer sem limite em sessões longas de auto-scan.

const maxImagensSessao = 10_000

var (
	imagensSessaoMu       sync.Mutex
	imagensSessao         = map[int]string{}
	ordemImagensSessao    []int // ids na ordem de inserção, para o descarte das mais antigas
	proximoIdImagemSessao = 1
)

// vistosSessao evita repetir o INSERT OR IGNORE das mesmas palavras a cada scan — para
// palavras já registradas o comando não muda nada, mas ainda custa um acesso a disco.
var (
	vistosSessaoMu sync.Mutex
	vistosSessao   = map[string]bool{}
)

func SalvarImagemSessao(base64 string) (int, error) {
	imagensSessaoMu.Lock()
	defer imagensSessaoMu.Unlock()

	id := proximoIdImagemSessao
	proximoIdImagemSessao++
	imagensSessao[id] = base64
	ordemImagensSessao = append(ordemImagensSessao, id)

	// Teto de RAM: descarta as imagens mais antigas (os cards antigos ficam sem crop no modal).
	for len(ordemImagensSessao) > maxImagensSessao {
		delete(imagensSessao, ordemImagensSessao[0])
		ordemImagensSessao = ordemImagensSessao[1:]
	}

	return id, nil
}

func GetImagemSessao(id int) (string, error) {
	imagensSessaoMu.Lock()
	defer imagensSessaoMu.Unlock()

	base64, achou := imagensSessao[id]
	if !achou {
		return "", fmt.Errorf("imagem de sessão %d não encontrada", id)
	}
	return base64, nil
}

func LimparImagensSessao() error {
	imagensSessaoMu.Lock()
	defer imagensSessaoMu.Unlock()

	imagensSessao = map[int]string{}
	ordemImagensSessao = nil
	return nil
}
