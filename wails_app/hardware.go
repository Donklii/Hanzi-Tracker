package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemHardware são os nomes reais de CPU e GPUs da máquina, para o select de hardware da UI.
// A lista de GPUs é informativa: a aceleração WebGPU usa o adaptador de vídeo padrão do sistema
// (o WebGpuExecutionProvider não expõe device_id), então escolher uma GPU específica só define a
// intenção CPU vs GPU — não qual placa executa.
type SystemHardware struct {
	Cpu  string   `json:"cpu"`
	Gpus []string `json:"gpus"`
}

// GetSystemHardware detecta os nomes de CPU/GPUs nativamente em Go, por SO (PowerShell/CIM no
// Windows; /proc/cpuinfo + lspci no Linux). Fica em arquivo próprio porque o app.go importa o
// runtime do Wails sem alias — aqui "runtime" é o da stdlib (runtime.GOOS, como em armazenamento.go).
func (a *App) GetSystemHardware() SystemHardware {
	var cpu string
	var gpus []string
	if runtime.GOOS == "windows" {
		cpu, gpus = hardwareWindows()
	} else {
		cpu, gpus = hardwareLinux()
	}

	if cpu == "" {
		cpu = "CPU"
	}
	if len(gpus) == 0 {
		gpus = append(gpus, "GPU (Detecção Falhou)")
	}
	return SystemHardware{Cpu: cpu, Gpus: gpus}
}

func hardwareWindows() (string, []string) {
	cpu := ""
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-ItemProperty -Path 'HKLM:\\HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0').ProcessorNameString").Output()
	if err == nil {
		cpu = strings.TrimSpace(string(out))
	}

	var linhas []string
	out, err = exec.Command("powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name").Output()
	if err == nil {
		linhas = strings.Split(string(out), "\n")
	}
	return cpu, filtrarGpus(linhas)
}

func hardwareLinux() (string, []string) {
	cpu := ""
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, linha := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(linha, "model name") {
				if _, nome, ok := strings.Cut(linha, ":"); ok {
					cpu = strings.TrimSpace(nome)
				}
				break
			}
		}
	}

	// lspci lista "xx:yy.z VGA compatible controller: <nome> (rev xx)" — também "3D controller"
	// (GPUs dedicadas em notebooks) e "Display controller" (algumas iGPUs).
	var linhas []string
	if out, err := exec.Command("lspci").Output(); err == nil {
		for _, linha := range strings.Split(string(out), "\n") {
			for _, classe := range []string{"VGA compatible controller: ", "3D controller: ", "Display controller: "} {
				if _, nome, ok := strings.Cut(linha, classe); ok {
					if idx := strings.LastIndex(nome, " (rev "); idx > 0 {
						nome = nome[:idx]
					}
					linhas = append(linhas, nome)
					break
				}
			}
		}
	}
	return cpu, filtrarGpus(linhas)
}

// filtrarGpus limpa a lista de adaptadores: descarta vazios e virtuais (a menos que só haja
// virtuais) e remove duplicatas preservando a ordem.
func filtrarGpus(linhas []string) []string {
	var todas []string
	var filtradas []string
	for _, linha := range linhas {
		linha = strings.TrimSpace(linha)
		if linha == "" {
			continue
		}
		todas = append(todas, linha)

		linhaLower := strings.ToLower(linha)
		virtual := false
		for _, excl := range []string{"virtual", "parsec", "mirror", "remote"} {
			if strings.Contains(linhaLower, excl) {
				virtual = true
				break
			}
		}
		if !virtual {
			filtradas = append(filtradas, linha)
		}
	}

	usar := filtradas
	if len(filtradas) == 0 {
		usar = todas
	}

	var gpus []string
	for _, g := range usar {
		existe := false
		for _, e := range gpus {
			if e == g {
				existe = true
				break
			}
		}
		if !existe {
			gpus = append(gpus, g)
		}
	}
	return gpus
}
