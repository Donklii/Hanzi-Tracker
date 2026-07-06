package baixador

import "runtime"

// ----- Nomes de artefato por SO -----
// Os catálogos de motores (motoresocr/motorestts) publicam um zip POR SISTEMA OPERACIONAL na mesma
// família de releases: os do Windows mantêm o nome histórico ("ocr_server.zip", exe com ".exe") e os
// do Linux ganham o sufixo "_linux" ("ocr_server_linux.zip", executável ELF sem sufixo — o padrão do
// PyInstaller fora do Windows). A escolha é por runtime.GOOS, sem build tags: o binário já é por-SO.

// NomeExecutavelSo devolve o nome do executável congelado de um sidecar para o SO atual
// (ex.: "ocr_server" → "ocr_server.exe" no Windows, "ocr_server" no Linux).
func NomeExecutavelSo(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// NomeZipArtefatoSo devolve o nome do zip publicado de um sidecar para o SO atual
// (ex.: "ocr_server" → "ocr_server.zip" no Windows, "ocr_server_linux.zip" no Linux).
func NomeZipArtefatoSo(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".zip"
	}
	return base + "_linux.zip"
}
