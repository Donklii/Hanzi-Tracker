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
	Id          int
	Hanzi       string
	Pinyin      string
	Significado string
	Status      string // "estudo", "aprendido"
	DataAdd     time.Time
	TipoHanzi   string `json:"tipoHanzi"`
}

var db *sql.DB

// vistosSessao evita repetir o INSERT OR IGNORE das mesmas palavras a cada scan — para
// palavras já registradas o comando não muda nada, mas ainda custa um acesso a disco.
var (
	vistosSessaoMu sync.Mutex
	vistosSessao   = map[string]bool{}
)

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

	// Imagens de sessão são efêmeras: descarta sobras da sessão anterior (shutdown abrupto) e
	// recupera o espaço em disco que elas ocupavam (o VACUUM copia só as páginas vivas — barato).
	if _, err = db.Exec("DELETE FROM session_images"); err != nil {
		return err
	}
	_, _ = db.Exec("VACUUM")
	return nil
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
		v.DataAdd = parseDataSqlite(d)
		lista = append(lista, v)
	}
	return lista, nil
}

// parseDataSqlite converte o texto de um DATETIME do SQLite em time.Time. O DEFAULT
// CURRENT_TIMESTAMP grava "2006-01-02 15:04:05" (UTC), NÃO RFC3339 — parsear como RFC3339
// devolvia sempre o time zero. Mantém o RFC3339 como fallback para valores gravados por
// drivers/versões que usem esse formato. Falha vira time zero (dado meramente informativo).
func parseDataSqlite(d string) time.Time {
	if t, err := time.Parse("2006-01-02 15:04:05", d); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, d)
	return t
}

// ----- Imagens de sessão (crops dos cards) -----
// Ficam em DISCO (tabela session_images), com leitura preguiçosa: o frontend guarda só o id e busca
// o base64 quando o card é aberto. Sessões não têm duração definida, então em RAM os crops de um
// auto-scan longo cresceriam sem limite (ou, com teto, os cards antigos perderiam o crop cedo).
// O custo de disco fica controlado gravando os crops de cada scan numa ÚNICA transação (um fsync
// por scan, não por palavra) e descartando as mais antigas acima do teto.

const maxImagensSessao = 1_000

// SalvarImagensSessaoLote grava todos os crops de um scan numa única transação e devolve os ids
// gerados, na mesma ordem. Também descarta as imagens mais antigas acima de maxImagensSessao
// (os cards antigos ficam sem crop no modal).
func SalvarImagensSessaoLote(imagens []string) ([]int, error) {
	if len(imagens) == 0 {
		return nil, nil
	}
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO session_images (image_base64) VALUES (?)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	ids := make([]int, 0, len(imagens))
	for _, base64 := range imagens {
		res, err := stmt.Exec(base64)
		if err != nil {
			return nil, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		ids = append(ids, int(id))
	}

	// Teto de disco: descarta as mais antigas (ids são monotônicos por AUTOINCREMENT).
	ultimoId := ids[len(ids)-1]
	if _, err := tx.Exec("DELETE FROM session_images WHERE id <= ?", ultimoId-maxImagensSessao); err != nil {
		return nil, err
	}

	return ids, tx.Commit()
}

func GetImagemSessao(id int) (string, error) {
	if db == nil {
		return "", fmt.Errorf("DB não inicializado")
	}

	var base64 string
	err := db.QueryRow("SELECT image_base64 FROM session_images WHERE id = ?", id).Scan(&base64)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("imagem de sessão %d não encontrada", id)
	}
	if err != nil {
		return "", err
	}
	return base64, nil
}

// LimparImagensSessao esvazia a tabela (chamada no shutdown; o espaço em disco é recuperado
// pelo VACUUM do próximo InitDB).
func LimparImagensSessao() error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	_, err := db.Exec("DELETE FROM session_images")
	return err
}
