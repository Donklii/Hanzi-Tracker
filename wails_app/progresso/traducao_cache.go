package progresso

import (
	"database/sql"
	"errors"
	"fmt"
)

// BuscarTraducaoCache procura uma tradução já armazenada para a linha original.
// Devolve a tradução, se achou, e um eventual erro de banco.
func BuscarTraducaoCache(linhaOriginal string) (traducao string, achou bool, err error) {
	if db == nil {
		return "", false, fmt.Errorf("DB não inicializado")
	}

	err = db.QueryRow(
		"SELECT traducao FROM traducoes_cache WHERE linha_original = ?",
		linhaOriginal,
	).Scan(&traducao)

	if err != nil {
		// sql.ErrNoRows não é um erro de fato — só indica cache miss.
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}

	return traducao, true, nil
}

// SalvarTraducaoCache armazena uma tradução no cache. Usa INSERT OR IGNORE para que
// a constraint UNIQUE sirva de rede de segurança contra corridas/duplicatas entre scans
// diferentes. O chamador DEVE ter verificado BuscarTraducaoCache ANTES (ver aviso no TODO.md).
func SalvarTraducaoCache(linhaOriginal, traducao string) error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}

	_, err := db.Exec(
		"INSERT OR IGNORE INTO traducoes_cache (linha_original, traducao) VALUES (?, ?)",
		linhaOriginal, traducao,
	)
	return err
}

// LimparCacheTraducao apaga todas as traduções cacheadas e recupera o espaço em disco.
func LimparCacheTraducao() error {
	if db == nil {
		return fmt.Errorf("DB não inicializado")
	}
	if _, err := db.Exec("DELETE FROM traducoes_cache"); err != nil {
		return err
	}
	// Recupera o espaço em disco liberado pelos registros apagados.
	_, _ = db.Exec("VACUUM")
	return nil
}

// TamanhoCacheTraducao devolve uma estimativa do tamanho em bytes e a contagem de linhas
// cacheadas. Como o cache é uma tabela dentro de progresso.db (não um arquivo próprio),
// o tamanho é aproximado via SUM(LENGTH(...)).
func TamanhoCacheTraducao() (bytes int64, linhas int, err error) {
	if db == nil {
		return 0, 0, fmt.Errorf("DB não inicializado")
	}

	// Contagem de linhas
	err = db.QueryRow("SELECT COUNT(*) FROM traducoes_cache").Scan(&linhas)
	if err != nil {
		return 0, 0, err
	}

	// Guard clause: tabela vazia — sem necessidade de calcular SUM
	if linhas == 0 {
		return 0, 0, nil
	}

	// Tamanho aproximado: soma dos comprimentos das colunas de texto
	err = db.QueryRow(
		"SELECT COALESCE(SUM(LENGTH(linha_original) + LENGTH(traducao)), 0) FROM traducoes_cache",
	).Scan(&bytes)
	if err != nil {
		return 0, linhas, err
	}

	return bytes, linhas, nil
}
