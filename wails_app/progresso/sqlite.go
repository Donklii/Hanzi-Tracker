package progresso

import (
	"database/sql"
	"encoding/json"
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

	// VezesVistaOcr conta em quantos scans de OCR a palavra apareceu com recorte inédito frente ao
	// scan anterior — palavra parada na tela não acumula (ver RegistrarVisualizacoesOcr e o
	// rastreador em visualizacoes_ocr.go do app).
	VezesVistaOcr  int `json:"vezesVistaOcr"`
	PosicaoRanking int `json:"posicaoRanking"`
}

var db *sql.DB

// EsperaMaximaLockBancoMs é quanto uma operação espera pelo lock do arquivo antes de desistir com
// SQLITE_BUSY. Na prática a espera real é de milissegundos (o tempo do outro comando terminar); o
// teto só existe para nunca travar a interface indefinidamente.
const EsperaMaximaLockBancoMs = 5000

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

	// busy_timeout no DSN (o driver aplica o PRAGMA em CADA conexão nova do pool): os bindings do
	// Wails rodam em goroutines próprias, então chamadas simultâneas abrem conexões diferentes e
	// disputam o lock do arquivo — ex.: ao concluir uma revisão da Jornada, a gravação do progresso
	// sai junto com as duas leituras do placar. Sem timeout o SQLite devolve SQLITE_BUSY na hora e a
	// escrita se perde; com timeout ela ESPERA o leitor terminar (milissegundos, na prática).
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(%d)", dbPath, EsperaMaximaLockBancoMs)

	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}

	query := `
	CREATE TABLE IF NOT EXISTS vocabulario (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hanzi TEXT UNIQUE,
		status TEXT,
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP,
		acertos_sequencia_significado INTEGER DEFAULT 0,
		acertos_sequencia_fonetica INTEGER DEFAULT 0,
		acertos_sequencia_desenho INTEGER DEFAULT 0,
		acertos_sequencia_contexto INTEGER DEFAULT 0,
		acertos_sequencia_pronuncia INTEGER DEFAULT 0,
		vezes_vista_ocr INTEGER DEFAULT 0,
		sugestao_estudo_ocultada INTEGER DEFAULT 0
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

	CREATE TABLE IF NOT EXISTS foco_revisao (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hanzi TEXT UNIQUE NOT NULL,
		data_entrada DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS frases_usuario (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chines TEXT UNIQUE NOT NULL,
		ingles TEXT NOT NULL,
		atribuicao TEXT NOT NULL,
		tema TEXT NOT NULL DEFAULT '',
		dificuldade TEXT NOT NULL DEFAULT '',
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS perguntas_compreensao (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		contexto TEXT UNIQUE NOT NULL,
		pergunta TEXT NOT NULL,
		opcoes TEXT NOT NULL,
		indice_resposta_correta INTEGER NOT NULL,
		pergunta_traduzida TEXT NOT NULL DEFAULT '',
		contexto_traduzido TEXT NOT NULL DEFAULT '',
		tema TEXT NOT NULL DEFAULT '',
		dificuldade TEXT NOT NULL DEFAULT '',
		atribuicao TEXT NOT NULL DEFAULT '',
		script TEXT NOT NULL DEFAULT 'simplified',
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS jornada_progresso (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nivel_id TEXT UNIQUE NOT NULL,
		revisoes_concluidas INTEGER NOT NULL DEFAULT 0,
		criado_em DATETIME DEFAULT CURRENT_TIMESTAMP,
		atualizado_em DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS recomendacoes_baralho (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hanzi TEXT UNIQUE NOT NULL,
		pinyin TEXT NOT NULL DEFAULT '',
		significado TEXT NOT NULL DEFAULT '',
		motivo TEXT NOT NULL DEFAULT '',
		revelada INTEGER NOT NULL DEFAULT 0,
		data_add DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	// Migração para remover colunas obsoletas se existirem
	if err := removerColunasObsoletas(); err != nil {
		return err
	}

	// Migração para adicionar colunas de estatísticas se não existirem
	if err := adicionarColunasEstatisticas(); err != nil {
		return err
	}

	if err := adicionarColunasFrasesUsuario(); err != nil {
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

func AddOuUpdateVocab(hanzi, status string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	query := `
	INSERT INTO vocabulario (hanzi, status) 
	VALUES (?, ?)
	ON CONFLICT(hanzi) DO UPDATE SET 
		status=excluded.status;
	`
	_, err := db.Exec(query, hanzi, status)
	return err
}

// RegistrarVisto auto-salva uma palavra como 'visto' se ela ainda não existir
func RegistrarVisto(hanzi string) error {
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
	INSERT OR IGNORE INTO vocabulario (hanzi, status) 
	VALUES (?, 'visto')
	`
	_, err := db.Exec(query, hanzi)

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

// ListarHanzisVocab devolve todas as grafias (chave `hanzi`) da tabela vocabulario. Alimenta a
// migração que conserta resíduos do antigo bug de conversão 么→幺 (ver main/migracao_vocabulario.go).
func ListarHanzisVocab() ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}
	rows, err := db.Query("SELECT hanzi FROM vocabulario")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hanzis []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hanzis = append(hanzis, h)
	}
	return hanzis, rows.Err()
}

// RenomearHanziVocab troca a chave `de` por `para` na tabela vocabulario, preservando o progresso.
// Quando `para` já existe (o usuário já tem a grafia correta), a linha `de` é apenas REMOVIDA — a
// correta fica com seu progresso, sem violar o UNIQUE(hanzi). Tudo numa transação.
func RenomearHanziVocab(de, para string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existe int
	if err := tx.QueryRow("SELECT COUNT(*) FROM vocabulario WHERE hanzi = ?", para).Scan(&existe); err != nil {
		return err
	}
	if existe > 0 {
		if _, err := tx.Exec("DELETE FROM vocabulario WHERE hanzi = ?", de); err != nil {
			return err
		}
	} else if _, err := tx.Exec("UPDATE vocabulario SET hanzi = ? WHERE hanzi = ?", para, de); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	vistosSessaoMu.Lock()
	delete(vistosSessao, de)
	vistosSessaoMu.Unlock()
	return nil
}

func GetAllVocab() ([]Vocab, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query(`
		SELECT ` + colunasVocab + `
		FROM vocabulario ORDER BY data_add DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return lerLinhasVocab(rows)
}

// colunasVocab é a lista de colunas que preenche um Vocab — fonte única compartilhada por todas as
// consultas que devolvem a struct, mantida em sincronia com lerLinhasVocab (a ordem importa).
const colunasVocab = `id, hanzi, status, data_add,
       acertos_sequencia_significado, acertos_sequencia_fonetica,
       acertos_sequencia_desenho, acertos_sequencia_contexto,
       acertos_sequencia_pronuncia, vezes_vista_ocr`

// lerLinhasVocab converte as linhas de uma consulta com colunasVocab em []Vocab.
func lerLinhasVocab(rows *sql.Rows) ([]Vocab, error) {
	var lista []Vocab
	for rows.Next() {
		var v Vocab
		var d string
		if err := rows.Scan(&v.Id, &v.Hanzi, &v.Status, &d,
			&v.AcertosSequenciaSignificado, &v.AcertosSequenciaFonetica,
			&v.AcertosSequenciaDesenho, &v.AcertosSequenciaContexto,
			&v.AcertosSequenciaPronuncia, &v.VezesVistaOcr); err != nil {
			return nil, err
		}
		v.DataAdd = parseDataSqlite(d)
		lista = append(lista, v)
	}
	return lista, rows.Err()
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

// removerColunasObsoletas verifica a estrutura da tabela vocabulario e remove as colunas pinyin e significado se existirem.
func removerColunasObsoletas() error {
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

	if colunasExistentes["pinyin"] {
		if _, err := db.Exec("ALTER TABLE vocabulario DROP COLUMN pinyin"); err != nil {
			return err
		}
	}
	if colunasExistentes["significado"] {
		if _, err := db.Exec("ALTER TABLE vocabulario DROP COLUMN significado"); err != nil {
			return err
		}
	}
	return nil
}

// ----- Seção: Estatísticas de Aprendizado e Sequências (Streaks) -----

// adicionarColunasEstatisticas verifica a estrutura da tabela vocabulario e adiciona as colunas
// criadas depois do schema original (acertos consecutivos, visualizações de OCR e o silenciamento
// da sugestão de estudo) se elas estiverem ausentes.
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
		"vezes_vista_ocr",
		"sugestao_estudo_ocultada",
	}

	// Coluna de timestamp (DATETIME): tratada separadamente por ter tipo diferente de INTEGER.
	if !colunasExistentes["ultima_pratica"] {
		if _, err := db.Exec("ALTER TABLE vocabulario ADD COLUMN ultima_pratica DATETIME"); err != nil {
			return err
		}
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

func adicionarColunasFrasesUsuario() error {
	rows, err := db.Query("PRAGMA table_info(frases_usuario)")
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

	colunasNovas := []string{"tema", "dificuldade"}
	for _, col := range colunasNovas {
		if !colunasExistentes[col] {
			_, err := db.Exec(fmt.Sprintf("ALTER TABLE frases_usuario ADD COLUMN %s TEXT NOT NULL DEFAULT ''", col))
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
func GarantirVocabExiste(hanzi string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	query := `
	INSERT OR IGNORE INTO vocabulario (hanzi, status)
	VALUES (?, 'visto')
	`
	_, err := db.Exec(query, hanzi)
	return err
}

// AtualizarAcertosSequencia incrementa em 1 ou reseta para 0 o streak de uma palavra em uma categoria.
func AtualizarAcertosSequencia(hanzi, categoria string, acertou bool) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	coluna, err := obterColunaCategoria(categoria)
	if err != nil {
		// Retorna nil sem erro se for uma categoria que não conta (ex: ordenação)
		return nil
	}

	if err := GarantirVocabExiste(hanzi); err != nil {
		return err
	}

	var query string
	if acertou {
		query = fmt.Sprintf("UPDATE vocabulario SET %s = MIN(%s + 1, 9), ultima_pratica = CURRENT_TIMESTAMP WHERE hanzi = ?", coluna, coluna)
	} else {
		query = fmt.Sprintf("UPDATE vocabulario SET %s = 0, ultima_pratica = CURRENT_TIMESTAMP WHERE hanzi = ?", coluna)
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

// ObterUltimaPratica devolve o timestamp da última prática da palavra, ou time.Time zero se nunca praticada.
func ObterUltimaPratica(hanzi string) (time.Time, error) {
	if db == nil {
		return time.Time{}, fmt.Errorf("DB não inicializado")
	}
	var ultima sql.NullTime
	err := db.QueryRow("SELECT ultima_pratica FROM vocabulario WHERE hanzi = ?", hanzi).Scan(&ultima)
	if err == sql.ErrNoRows || !ultima.Valid {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return ultima.Time, nil
}

// ObterSugestoesAprendidoLote verifica quais das palavras enviadas atingiram o critério para serem marcadas como aprendidas nas categorias habilitadas.
func ObterSugestoesAprendidoLote(hanzis []string, modos []string) ([]Vocab, error) {
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

	condicoes := []string{
		"status != 'aprendido'",
		fmt.Sprintf("hanzi IN (%s)", strings.Join(placeholders, ",")),
	}

	if len(modos) == 0 {
		modos = []string{"significado", "fonetica", "desenho", "contexto", "pronuncia"}
	}
	modosSet := make(map[string]bool, len(modos))
	for _, m := range modos {
		modosSet[m] = true
	}

	colunasModos := map[string]string{
		"significado": "acertos_sequencia_significado >= 3",
		"fonetica":    "acertos_sequencia_fonetica >= 3",
		"desenho":     "acertos_sequencia_desenho >= 3",
		"contexto":    "acertos_sequencia_contexto >= 3",
		"pronuncia":   "acertos_sequencia_pronuncia >= 3",
	}
	for modo, cond := range colunasModos {
		if modosSet[modo] {
			condicoes = append(condicoes, cond)
		}
	}

	query := fmt.Sprintf(`
		SELECT `+colunasVocab+`
		FROM vocabulario
		WHERE %s
	`, strings.Join(condicoes, " AND "))

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return lerLinhasVocab(rows)
}

// ----- Seção: Foco de revisão (grupo global de caracteres priorizados) -----
// A tabela foco_revisao guarda o grupo de caracteres que as sessões de revisão priorizam até
// serem marcados como aprendidos. A lógica de quem entra/sai fica no app (foco_revisao.go);
// aqui só a persistência. Por morar no banco de progresso, o grupo viaja junto no snapshot
// da sincronização de nuvem.

// ObterFoco devolve os caracteres do grupo de foco na ordem de entrada (mais antigo primeiro).
func ObterFoco() ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query("SELECT hanzi FROM foco_revisao ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foco []string
	for rows.Next() {
		var hanzi string
		if err := rows.Scan(&hanzi); err != nil {
			return nil, err
		}
		foco = append(foco, hanzi)
	}
	return foco, rows.Err()
}

// ----- Frases do usuário (geradas pela geração de frases com IA e salvas para reuso na revisão geral) -----

// FraseUsuario espelha uma linha de frases_usuario. Os campos casam com dicionario.Frase, mas este
// pacote não importa dicionario — a conversão fica a cargo do chamador (main).
type FraseUsuario struct {
	Chines      string
	Ingles      string
	Atribuicao  string
	Tema        string
	Dificuldade string
}

// AddFraseUsuario grava uma frase gerada pela IA. Devolve true se a frase era inédita (inserida) e
// false se já existia (INSERT OR IGNORE não altera linhas). Idempotente pela restrição UNIQUE.
func AddFraseUsuario(chines, ingles, atribuicao, tema, dificuldade string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("DB não inicializado")
	}
	res, err := db.Exec(
		"INSERT OR IGNORE INTO frases_usuario (chines, ingles, atribuicao, tema, dificuldade) VALUES (?, ?, ?, ?, ?)",
		chines, ingles, atribuicao, tema, dificuldade,
	)
	if err != nil {
		return false, err
	}
	linhas, _ := res.RowsAffected()
	return linhas > 0, nil
}

// GetFrasesUsuario devolve todas as frases salvas pelo usuário, na ordem de inserção.
func GetFrasesUsuario() ([]FraseUsuario, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query("SELECT chines, ingles, atribuicao, tema, dificuldade FROM frases_usuario ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var frases []FraseUsuario
	for rows.Next() {
		var f FraseUsuario
		if err := rows.Scan(&f.Chines, &f.Ingles, &f.Atribuicao, &f.Tema, &f.Dificuldade); err != nil {
			return nil, err
		}
		frases = append(frases, f)
	}
	return frases, rows.Err()
}

// RemoverFraseUsuario exclui uma frase gerada pela IA a partir do texto chinês.
func RemoverFraseUsuario(chines string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("DB não inicializado")
	}
	res, err := db.Exec("DELETE FROM frases_usuario WHERE chines = ?", chines)
	if err != nil {
		return false, err
	}
	linhas, _ := res.RowsAffected()
	return linhas > 0, nil
}

// ----- Perguntas de Compreensão (geradas por IA e salvas no banco SQLite) -----

// PerguntaCompreensaoUsuario espelha uma linha de perguntas_compreensao.
type PerguntaCompreensaoUsuario struct {
	Contexto              string   `json:"contexto"`
	Pergunta              string   `json:"pergunta"`
	Opcoes                []string `json:"opcoes"`
	IndiceRespostaCorreta int      `json:"indice_resposta_correta"`
	PerguntaTraduzida     string   `json:"pergunta_traduzida"`
	ContextoTraduzido     string   `json:"contexto_traduzido"`
	Tema                  string   `json:"tema"`
	Dificuldade           string   `json:"dificuldade"`
	Atribuicao            string   `json:"atribuicao"`
	Script                string   `json:"script"`
}

// AddPerguntaCompreensao grava uma pergunta de compreensão gerada por IA no banco.
func AddPerguntaCompreensao(contexto, pergunta string, opcoes []string, indiceCorreto int, perguntaTrad, contextoTrad, tema, dificuldade, atribuicao, script string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("DB não inicializado")
	}
	opcoesJSON, err := json.Marshal(opcoes)
	if err != nil {
		return false, fmt.Errorf("falha ao serializar opções: %w", err)
	}

	res, err := db.Exec(
		`INSERT OR IGNORE INTO perguntas_compreensao 
		(contexto, pergunta, opcoes, indice_resposta_correta, pergunta_traduzida, contexto_traduzido, tema, dificuldade, atribuicao, script) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		contexto, pergunta, string(opcoesJSON), indiceCorreto, perguntaTrad, contextoTrad, tema, dificuldade, atribuicao, script,
	)
	if err != nil {
		return false, err
	}
	linhas, _ := res.RowsAffected()
	return linhas > 0, nil
}

// GetPerguntasCompreensao devolve todas as perguntas de compreensão do usuário salvas no banco.
func GetPerguntasCompreensao() ([]PerguntaCompreensaoUsuario, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query("SELECT contexto, pergunta, opcoes, indice_resposta_correta, pergunta_traduzida, contexto_traduzido, tema, dificuldade, atribuicao, script FROM perguntas_compreensao ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []PerguntaCompreensaoUsuario
	for rows.Next() {
		var p PerguntaCompreensaoUsuario
		var opcoesJSON string
		if err := rows.Scan(&p.Contexto, &p.Pergunta, &opcoesJSON, &p.IndiceRespostaCorreta, &p.PerguntaTraduzida, &p.ContextoTraduzido, &p.Tema, &p.Dificuldade, &p.Atribuicao, &p.Script); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(opcoesJSON), &p.Opcoes)
		lista = append(lista, p)
	}
	return lista, rows.Err()
}

// RemoverPerguntaCompreensao exclui uma pergunta de compreensão pelo contexto.
func RemoverPerguntaCompreensao(contexto string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("DB não inicializado")
	}
	res, err := db.Exec("DELETE FROM perguntas_compreensao WHERE contexto = ?", contexto)
	if err != nil {
		return false, err
	}
	linhas, _ := res.RowsAffected()
	return linhas > 0, nil
}

// AtualizarFoco aplica a rotação do grupo numa única transação: remove os que saíram e insere
// os que entraram (no fim da fila — a ordem de entrada é preservada pelo AUTOINCREMENT).
func AtualizarFoco(remover, adicionar []string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if len(remover) == 0 && len(adicionar) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, hanzi := range remover {
		if _, err := tx.Exec("DELETE FROM foco_revisao WHERE hanzi = ?", hanzi); err != nil {
			return err
		}
	}
	for _, hanzi := range adicionar {
		if _, err := tx.Exec("INSERT OR IGNORE INTO foco_revisao (hanzi) VALUES (?)", hanzi); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ----- Seção: Visualizações de OCR e sugestão de estudo -----
// Cada scan de OCR soma +1 visualização às palavras que apareceram na tela (dedup por scan; telas
// idênticas nem chegam aqui — o hash da captura descarta o rescan). O contador alimenta o peso do
// sorteio do grupo de foco (wails_app/revisao/foco.go) e o pop-up que sugere mover para "em estudo"
// as palavras 'vistas' que mais aparecem. `sugestao_estudo_ocultada` persiste o "não sugerir de novo".

const (
	// VezesVistaMinimaSugestaoEstudo é o mínimo de visualizações de OCR para uma palavra 'vista'
	// entrar no pop-up de sugestão de estudo.
	VezesVistaMinimaSugestaoEstudo = 5

	// MaximoSugestoesEstudoOcr limita o "baralho" do pop-up às palavras mais vistas.
	MaximoSugestoesEstudoOcr = 8
)

// RegistrarVisualizacoesOcr soma +1 visualização a cada palavra recebida, numa única transação
// (um fsync por scan, não por palavra). Quem decide QUAIS palavras do scan contam é o chamador
// (rastreador de crops em visualizacoes_ocr.go do app). Palavras ainda sem linha no banco não são
// afetadas pelo UPDATE — o RegistrarVisto do próprio scan as cria antes do incremento.
func RegistrarVisualizacoesOcr(palavras []string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if len(palavras) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE vocabulario SET vezes_vista_ocr = MIN(vezes_vista_ocr + 1, 9999) WHERE hanzi = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, palavra := range palavras {
		if _, err := stmt.Exec(palavra); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IncrementarVisualizacoesVocab incrementa o contador vezes_vista_ocr de cada palavra pelo valor especificado.
func IncrementarVisualizacoesVocab(ocorrencias map[string]int) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if len(ocorrencias) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE vocabulario SET vezes_vista_ocr = MIN(vezes_vista_ocr + ?, 9999) WHERE hanzi = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for palavra, qtd := range ocorrencias {
		if _, err := stmt.Exec(qtd, palavra); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ObterSugestoesEstudoOcr devolve as palavras 'vistas' que o OCR mais encontrou (da mais vista para
// a menos), já filtrando quem não atingiu o mínimo, quem o usuário silenciou e quem já está em
// estudo/aprendido (o status 'visto' garante isso).
func ObterSugestoesEstudoOcr() ([]Vocab, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query(`
		SELECT `+colunasVocab+`
		FROM vocabulario
		WHERE status = 'visto'
		  AND vezes_vista_ocr >= ?
		  AND sugestao_estudo_ocultada = 0
		ORDER BY vezes_vista_ocr DESC
		LIMIT 50
	`, VezesVistaMinimaSugestaoEstudo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return lerLinhasVocab(rows)
}

// OcultarSugestoesEstudoOcr atende o "não exibir novamente" do pop-up: as palavras marcadas saem
// das sugestões para sempre (o contador de visualizações continua subindo — só a sugestão silencia).
func OcultarSugestoesEstudoOcr(hanzis []string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if len(hanzis) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE vocabulario SET sugestao_estudo_ocultada = 1 WHERE hanzi = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, hanzi := range hanzis {
		if _, err := stmt.Exec(hanzi); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ----- Progresso da Jornada de Revisão -----
// A tabela guarda só o BRUTO: quantas revisões de cada nível o usuário concluiu. Nível concluído,
// ramo desbloqueado e nível atual são derivados em runtime a partir da árvore (revisao/jornada) —
// persistir o calculável só criaria estado para dessincronizar quando o conteúdo mudar.

// ObterProgressoJornada devolve o mapa nivel_id -> revisões concluídas.
func ObterProgressoJornada() (map[string]int, error) {
	if db == nil {
		return nil, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query("SELECT nivel_id, revisoes_concluidas FROM jornada_progresso")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	progresso := make(map[string]int)
	for rows.Next() {
		var nivelId string
		var concluidas int
		if err := rows.Scan(&nivelId, &concluidas); err != nil {
			return nil, err
		}
		progresso[nivelId] = concluidas
	}
	return progresso, rows.Err()
}

// RegistrarRevisaoJornada avança o contador do nível em uma revisão. O índice é conferido no próprio
// UPDATE: só incrementa quando revIndex é EXATAMENTE a revisão que faltava, então repetir uma revisão
// já feita (ou pular um índice) não mexe no progresso.
func RegistrarRevisaoJornada(nivelId string, revIndex int) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if nivelId == "" {
		return fmt.Errorf("nivelId vazio")
	}
	if revIndex < 0 {
		return fmt.Errorf("revIndex inválido: %d", revIndex)
	}

	if _, err := db.Exec("INSERT OR IGNORE INTO jornada_progresso (nivel_id) VALUES (?)", nivelId); err != nil {
		return err
	}

	_, err := db.Exec(`
		UPDATE jornada_progresso
		SET revisoes_concluidas = revisoes_concluidas + 1,
		    atualizado_em = CURRENT_TIMESTAMP
		WHERE nivel_id = ? AND revisoes_concluidas = ?
	`, nivelId, revIndex)
	return err
}

// ----- Recomendações do Baralho na Seção de Estudo -----

type ItemRecomendacaoBaralho struct {
	Hanzi       string    `json:"hanzi"`
	Pinyin      string    `json:"pinyin"`
	Significado string    `json:"significado"`
	Motivo      string    `json:"motivo"`
	Revelada    bool      `json:"revelada"`
	DataAdd     time.Time `json:"dataAdd"`
}

func ObterRecomendacoesBaralhoBanco() ([]ItemRecomendacaoBaralho, time.Time, error) {
	if db == nil {
		return nil, time.Time{}, fmt.Errorf("DB não inicializado")
	}

	rows, err := db.Query("SELECT hanzi, pinyin, significado, motivo, revelada, data_add FROM recomendacoes_baralho ORDER BY id ASC")
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var lista []ItemRecomendacaoBaralho
	var dataMaisRecente time.Time

	for rows.Next() {
		var item ItemRecomendacaoBaralho
		var reveladaInt int
		var dataStr string
		if err := rows.Scan(&item.Hanzi, &item.Pinyin, &item.Significado, &item.Motivo, &reveladaInt, &dataStr); err != nil {
			return nil, time.Time{}, err
		}
		item.Revelada = (reveladaInt == 1)
		item.DataAdd = parseDataSqlite(dataStr)
		if dataMaisRecente.IsZero() || item.DataAdd.After(dataMaisRecente) {
			dataMaisRecente = item.DataAdd
		}
		lista = append(lista, item)
	}

	return lista, dataMaisRecente, rows.Err()
}

func SalvarRecomendacoesBaralhoBanco(itens []ItemRecomendacaoBaralho) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM recomendacoes_baralho"); err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO recomendacoes_baralho (hanzi, pinyin, significado, motivo, revelada) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range itens {
		reveladaInt := 0
		if item.Revelada {
			reveladaInt = 1
		}
		if _, err := stmt.Exec(item.Hanzi, item.Pinyin, item.Significado, item.Motivo, reveladaInt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func AtualizarReveladaBaralhoBanco(hanzi string, revelada bool) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	reveladaInt := 0
	if revelada {
		reveladaInt = 1
	}
	_, err := db.Exec("UPDATE recomendacoes_baralho SET revelada = ? WHERE hanzi = ?", reveladaInt, hanzi)
	return err
}

func LimparRecomendacoesBaralhoBanco() error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	_, err := db.Exec("DELETE FROM recomendacoes_baralho")
	return err
}
