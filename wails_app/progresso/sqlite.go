package progresso

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Estatísticas de progresso
	AcertosSequenciaSignificado int `json:"acertosSequenciaSignificado"`
	AcertosSequenciaFonetica    int `json:"acertosSequenciaFonetica"`
	AcertosSequenciaDesenho     int `json:"acertosSequenciaDesenho"`
	AcertosSequenciaContexto    int `json:"acertosSequenciaContexto"`
	AcertosSequenciaPronuncia   int `json:"acertosSequenciaPronuncia"`
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
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP,
		acertos_sequencia_significado INTEGER DEFAULT 0,
		acertos_sequencia_fonetica INTEGER DEFAULT 0,
		acertos_sequencia_desenho INTEGER DEFAULT 0,
		acertos_sequencia_contexto INTEGER DEFAULT 0,
		acertos_sequencia_pronuncia INTEGER DEFAULT 0
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

	// Migração para adicionar colunas de estatísticas se não existirem
	if err := adicionarColunasEstatisticas(); err != nil {
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

	rows, err := db.Query(`
		SELECT id, hanzi, pinyin, significado, status, data_add,
		       acertos_sequencia_significado, acertos_sequencia_fonetica,
		       acertos_sequencia_desenho, acertos_sequencia_contexto,
		       acertos_sequencia_pronuncia
		FROM vocabulario ORDER BY data_add DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []Vocab
	for rows.Next() {
		var v Vocab
		var d string
		if err := rows.Scan(&v.Id, &v.Hanzi, &v.Pinyin, &v.Significado, &v.Status, &d,
			&v.AcertosSequenciaSignificado, &v.AcertosSequenciaFonetica,
			&v.AcertosSequenciaDesenho, &v.AcertosSequenciaContexto,
			&v.AcertosSequenciaPronuncia); err != nil {
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

// ----- Snapshot e troca do arquivo (sincronização de nuvem) -----

// ExportarSnapshot grava uma cópia consistente do banco em `destino` via VACUUM INTO — segura
// mesmo com o app escrevendo em paralelo (o SQLite serializa) e já compactada (só páginas vivas).
// É o que a sincronização de nuvem envia, em vez do arquivo vivo (que poderia ir pela metade).
func ExportarSnapshot(destino string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	// VACUUM INTO recusa destino existente; descarta a sobra de uma execução interrompida.
	if err := os.Remove(destino); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err := db.Exec("VACUUM INTO ?", destino)
	return err
}

// FecharDB fecha a conexão para o arquivo do banco poder ser substituído (sincronização de nuvem,
// escolha "usar dados da nuvem"). Reabrir com InitDB.
func FecharDB() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil

	// O cache de "vistos" descreve o banco antigo — o novo arquivo pode não ter essas palavras.
	vistosSessaoMu.Lock()
	vistosSessao = map[string]bool{}
	vistosSessaoMu.Unlock()

	return err
}

// ----- Seção: Estatísticas de Aprendizado e Sequências (Streaks) -----

// adicionarColunasEstatisticas verifica a estrutura da tabela vocabulario e adiciona as novas
// colunas de acertos consecutivos se elas estiverem ausentes.
func adicionarColunasEstatisticas() error {
	rows, err := db.Query("PRAGMA table_info(vocabulario)")
	if err != nil {
		return err
	}
	defer rows.Close()

	colunasExistentes := make(map[string]bool)
	for rows.Next() {
		var cid, notnull, pk int
		var nome, tipo string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &nome, &tipo, &notnull, &dflt, &pk); err != nil {
			return err
		}
		colunasExistentes[nome] = true
	}

	colunasNovas := []string{
		"acertos_sequencia_significado",
		"acertos_sequencia_fonetica",
		"acertos_sequencia_desenho",
		"acertos_sequencia_contexto",
		"acertos_sequencia_pronuncia",
	}

	for _, col := range colunasNovas {
		if !colunasExistentes[col] {
			_, err := db.Exec(fmt.Sprintf("ALTER TABLE vocabulario ADD COLUMN %s INTEGER DEFAULT 0", col))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// obterColunaCategoria mapeia o nome da categoria vinda da revisão para o nome da coluna no SQLite.
func obterColunaCategoria(categoria string) (string, error) {
	switch categoria {
	case "significado":
		return "acertos_sequencia_significado", nil
	case "fonetica":
		return "acertos_sequencia_fonetica", nil
	case "desenho":
		return "acertos_sequencia_desenho", nil
	case "contexto":
		return "acertos_sequencia_contexto", nil
	case "pronuncia":
		return "acertos_sequencia_pronuncia", nil
	}
	return "", fmt.Errorf("categoria sem coluna de estatística ou inválida: %s", categoria)
}

// GarantirVocabExiste assegura que um caractere existe na tabela vocabulario (como status 'visto' por padrão).
func GarantirVocabExiste(hanzi, pinyin, significado string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	query := `
	INSERT OR IGNORE INTO vocabulario (hanzi, pinyin, significado, status)
	VALUES (?, ?, ?, 'visto')
	`
	_, err := db.Exec(query, hanzi, pinyin, significado)
	return err
}

// AtualizarAcertosSequencia incrementa em 1 ou reseta para 0 o streak de uma palavra em uma categoria.
func AtualizarAcertosSequencia(hanzi, pinyin, significado, categoria string, acertou bool) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	coluna, err := obterColunaCategoria(categoria)
	if err != nil {
		// Retorna nil sem erro se for uma categoria que não conta (ex: ordenação)
		return nil
	}

	if err := GarantirVocabExiste(hanzi, pinyin, significado); err != nil {
		return err
	}

	var query string
	if acertou {
		query = fmt.Sprintf("UPDATE vocabulario SET %s = %s + 1 WHERE hanzi = ?", coluna, coluna)
	} else {
		query = fmt.Sprintf("UPDATE vocabulario SET %s = 0 WHERE hanzi = ?", coluna)
	}

	_, err = db.Exec(query, hanzi)
	return err
}

// ObterEstatisticasPalavra retorna o streak de cada uma das 5 categorias para o caractere.
func ObterEstatisticasPalavra(hanzi string) (map[string]int, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	var significado, fonetica, desenho, contexto, pronuncia int
	query := `
		SELECT acertos_sequencia_significado, acertos_sequencia_fonetica,
		       acertos_sequencia_desenho, acertos_sequencia_contexto,
		       acertos_sequencia_pronuncia
		FROM vocabulario WHERE hanzi = ?
	`
	err := db.QueryRow(query, hanzi).Scan(&significado, &fonetica, &desenho, &contexto, &pronuncia)
	if err == sql.ErrNoRows {
		return map[string]int{
			"significado": 0,
			"fonetica":    0,
			"desenho":     0,
			"contexto":    0,
			"pronuncia":   0,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return map[string]int{
		"significado": significado,
		"fonetica":    fonetica,
		"desenho":     desenho,
		"contexto":    contexto,
		"pronuncia":   pronuncia,
	}, nil
}

// ObterSugestoesAprendidoLote verifica quais das palavras enviadas atingiram o critério para serem marcadas como aprendidas.
func ObterSugestoesAprendidoLote(hanzis []string) ([]Vocab, error) {
	if len(hanzis) == 0 {
		return nil, nil
	}
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	// Constrói os placeholders para a query
	placeholders := make([]string, len(hanzis))
	args := make([]any, len(hanzis))
	for i, h := range hanzis {
		placeholders[i] = "?"
		args[i] = h
	}

	query := fmt.Sprintf(`
		SELECT id, hanzi, pinyin, significado, status, data_add,
		       acertos_sequencia_significado, acertos_sequencia_fonetica,
		       acertos_sequencia_desenho, acertos_sequencia_contexto,
		       acertos_sequencia_pronuncia
		FROM vocabulario
		WHERE status != 'aprendido'
		  AND hanzi IN (%s)
		  AND acertos_sequencia_significado >= 3
		  AND acertos_sequencia_fonetica >= 3
		  AND acertos_sequencia_desenho >= 3
		  AND acertos_sequencia_contexto >= 3
		  AND acertos_sequencia_pronuncia >= 3
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sugestoes []Vocab
	for rows.Next() {
		var v Vocab
		var d string
		if err := rows.Scan(&v.Id, &v.Hanzi, &v.Pinyin, &v.Significado, &v.Status, &d,
			&v.AcertosSequenciaSignificado, &v.AcertosSequenciaFonetica,
			&v.AcertosSequenciaDesenho, &v.AcertosSequenciaContexto,
			&v.AcertosSequenciaPronuncia); err != nil {
			return nil, err
		}
		v.DataAdd = parseDataSqlite(d)
		sugestoes = append(sugestoes, v)
	}

	return sugestoes, nil
}
