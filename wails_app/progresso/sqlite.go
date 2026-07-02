package progresso

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	`
	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	// Limpa as imagens da última sessão
	_, err = db.Exec("DELETE FROM session_images")
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

	// INSERT OR IGNORE won't update the status if the word is already 'estudo' or 'aprendido'
	query := `
	INSERT OR IGNORE INTO vocabulario (hanzi, pinyin, significado, status) 
	VALUES (?, ?, ?, 'visto')
	`
	_, err := db.Exec(query, hanzi, pinyin, significado)
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
	return nil
}

func RemoveVocab(hanzi string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	_, err := db.Exec("DELETE FROM vocabulario WHERE hanzi = ?", hanzi)
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

func SalvarImagemSessao(base64 string) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("DB não inicializado")
	}

	result, err := db.Exec("INSERT INTO session_images (image_base64) VALUES (?)", base64)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func GetImagemSessao(id int) (string, error) {
	if db == nil {
		return "", fmt.Errorf("DB não inicializado")
	}

	var base64 string
	err := db.QueryRow("SELECT image_base64 FROM session_images WHERE id = ?", id).Scan(&base64)
	if err != nil {
		return "", err
	}
	return base64, nil
}

func LimparImagensSessao() error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	_, err := db.Exec("DELETE FROM session_images")
	return err
}
