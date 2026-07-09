// ----- Seção: Configurações — catálogos de modelos e motores -----
// Dono do estado dos três catálogos (modelos de OCR, motores de OCR e motores de voz/TTS) e das
// ações que os manipulam: listar, baixar, remover, trocar o ativo e pré-carregar o cache de áudio.
//
// Os três catálogos compartilham o mesmo fluxo de baixar/remover (progresso → sucesso → erro),
// mudando só a API do Go e os states de destino — por isso CriarAcoesDeCatalogo é uma fábrica.
//
// O hook também guarda a matriz de compatibilidade Motor x Hardware: o hardware de processamento
// depende do motor ativo, então trocar de motor pode forçar a config a migrar para uma combinação
// suportada (avisando o usuário).
import { Dispatch, SetStateAction, useEffect, useState } from 'react';
import { config, main } from '../../wailsjs/go/models';
import {
  ListarModelos, BaixarModelo, RemoverModelo,
  ListarMotores, BaixarMotor, RemoverMotor, TrocarMotor,
  ListarMotoresTts, BaixarMotorTts, RemoverMotorTts,
  ListarMotoresStt, BaixarMotorStt, RemoverMotorStt,
  PreCarregarCacheTts, PararPreCacheTts,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { ProgressoPreCacheTts } from '../comum/tipos';

// Nomes de hardware/API trocados com o Go (config.hardwareSelecionado e config.dispositivoOcr).
const HARDWARE_CPU = 'CPU';
const DISPOSITIVO_CPU = 'cpu';
const DISPOSITIVO_WEBGPU = 'webgpu';
// Um motor declara na `variante` se acelera por GPU (ex.: "CPU/WebGPU").
const VARIANTE_WEBGPU = 'webgpu';


// Estado inicial/final do lote de síntese — usado ao iniciar e ao abortar por erro.
function criarProgressoPreCache(mensagem: string, emAndamento: boolean): ProgressoPreCacheTts {
  return { total: 0, processados: 0, sintetizados: 0, jaEmCache: 0, falhas: 0, emAndamento, mensagem };
}


interface OpcoesUseCatalogos {
  configuracoesApp: config.Config | null;
  infoHardware: main.SystemHardware | null;
  AplicarConfiguracao: (mudancas: Partial<config.Config>) => void;
}


export function useCatalogos({ configuracoesApp, infoHardware, AplicarConfiguracao }: OpcoesUseCatalogos) {
  const [modelos, setModelos] = useState<main.ModeloOcrInfo[]>([]);
  const [progressoModelo, setProgressoModelo] = useState<Record<string, string>>({});
  const [baixandoModelo, setBaixandoModelo] = useState<string | null>(null);

  const [motores, setMotores] = useState<main.MotorOcrInfo[]>([]);
  const [progressoMotor, setProgressoMotor] = useState<Record<string, string>>({});
  const [baixandoMotor, setBaixandoMotor] = useState<string | null>(null);
  const [trocandoMotor, setTrocandoMotor] = useState<string | null>(null);

  const [motoresTts, setMotoresTts] = useState<main.MotorTtsInfo[]>([]);
  const [progressoMotorTts, setProgressoMotorTts] = useState<Record<string, string>>({});
  const [baixandoMotorTts, setBaixandoMotorTts] = useState<string | null>(null);
  const [progressoPreCacheTts, setProgressoPreCacheTts] = useState<ProgressoPreCacheTts | null>(null);

  const [motoresStt, setMotoresStt] = useState<main.MotorSttInfo[]>([]);
  const [progressoMotorStt, setProgressoMotorStt] = useState<Record<string, string>>({});
  const [baixandoMotorStt, setBaixandoMotorStt] = useState<string | null>(null);

  const [avisoCompatibilidade, setAvisoCompatibilidade] = useState<string | null>(null);

  // Carga inicial + eventos vindos do Go. Registrado uma vez: os handlers só chamam setState e as
  // funções Carregar*, que não dependem de nada além dos setters (estáveis entre renders).
  useEffect(() => {
    CarregarModelos();
    CarregarMotores();
    CarregarMotoresTts();
    CarregarMotoresStt();

    EventsOn('modelo_download_progresso', (data: any) => {
      if (!data?.nome) return;
      if (data.mensagem) {
        setProgressoModelo(prev => ({ ...prev, [data.nome]: data.mensagem }));
      } else if (data.erro) {
        setProgressoModelo(prev => ({ ...prev, [data.nome]: '⚠️ ' + data.erro }));
      }
    });

    // Motores (sidecars): progresso de download/instalação. O evento é compartilhado entre motores
    // de OCR, de Voz e de Escuta (nomes não colidem entre os catálogos), então a mensagem alimenta
    // os três mapas — cada lista só renderiza os nomes do próprio catálogo.
    EventsOn('motor_download_progresso', (data: any) => {
      if (!data?.nome) return;
      if (data.mensagem) {
        setProgressoMotor(prev => ({ ...prev, [data.nome]: data.mensagem }));
        setProgressoMotorTts(prev => ({ ...prev, [data.nome]: data.mensagem }));
        setProgressoMotorStt(prev => ({ ...prev, [data.nome]: data.mensagem }));
      } else if (data.erro) {
        setProgressoMotor(prev => ({ ...prev, [data.nome]: '⚠️ ' + data.erro }));
        setProgressoMotorTts(prev => ({ ...prev, [data.nome]: '⚠️ ' + data.erro }));
        setProgressoMotorStt(prev => ({ ...prev, [data.nome]: '⚠️ ' + data.erro }));
      }
    });

    // Pré-carregamento do cache de áudio: andamento do lote de síntese (barra de progresso na aba
    // Motores → TTS). emAndamento=false marca o fim (concluído, cancelado ou abortado).
    EventsOn('tts_precache_progresso', (prog: ProgressoPreCacheTts) => {
      setProgressoPreCacheTts(prog);
    });

    // Motor ativo mudou (bootstrap ou pronto): a lista de modelos vem do /api/modelos do motor em
    // execução, então precisa recarregar junto — senão fica mostrando o catálogo do motor anterior.
    EventsOn('ocr_pronto', () => { CarregarMotores(); CarregarModelos(); });
    EventsOn('motor_bootstrap_fim', () => { CarregarMotores(); CarregarModelos(); });
  }, []);

  // Garante que o modelo de OCR selecionado sempre seja compatível com o motor ativo. A lista de
  // `modelos` vem do /api/modelos do PROCESSO ativo (ver CarregarModelos), então ao trocar de motor
  // ela passa a refletir outro catálogo — mas modeloOcr, salvo na config, ainda aponta para um modelo
  // do motor ANTERIOR. Corrige automaticamente para um modelo compatível (priorizando o embutido, que
  // já está pronto pra uso sem download) assim que a nova lista chega.
  useEffect(() => {
    if (!configuracoesApp || modelos.length === 0) return;

    const modelosDisponiveis = modelos.filter(m => m.embutido || m.instalado);
    const atualCompativel = modelosDisponiveis.some(m => m.nome === configuracoesApp.modeloOcr);
    if (atualCompativel) return;

    const substituto = modelosDisponiveis.find(m => m.embutido) || modelosDisponiveis[0];
    if (substituto) {
      AplicarConfiguracao({ modeloOcr: substituto.nome });
    }
    // Se não houver nenhum modelo disponível (ex.: EasyOCR recém-ativado, sem download prévio), não
    // há para onde migrar — o seletor de Modelo de OCR mostra "indisponível" até o usuário baixar um.
  }, [modelos]);

  // Fábrica dos pares baixar/remover de itens de catálogo (modelos de OCR, motores de OCR e de
  // voz): os três compartilham o mesmo fluxo de progresso/sucesso/erro, mudando só a API chamada e
  // os states de destino.
  const CriarAcoesDeCatalogo = (
    api: { baixar: (nome: string) => Promise<void>; remover: (nome: string) => Promise<void> },
    setProgresso: Dispatch<SetStateAction<Record<string, string>>>,
    setBaixando: Dispatch<SetStateAction<string | null>>,
    recarregar: () => void,
  ) => ({
    baixar: (nome: string) => {
      setBaixando(nome);
      setProgresso(prev => ({ ...prev, [nome]: 'Iniciando download…' }));
      api.baixar(nome)
        .then(() => {
          setProgresso(prev => ({ ...prev, [nome]: '✅ Instalado!' }));
          recarregar();
        })
        .catch((err: any) => {
          setProgresso(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
        })
        .finally(() => setBaixando(null));
    },
    remover: (nome: string) => {
      api.remover(nome)
        .then(() => {
          setProgresso(prev => {
            const copia = { ...prev };
            delete copia[nome];
            return copia;
          });
          recarregar();
        })
        .catch((err: any) => {
          setProgresso(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
        });
    },
  });

  const acoesModeloOcr = CriarAcoesDeCatalogo(
    { baixar: BaixarModelo, remover: RemoverModelo }, setProgressoModelo, setBaixandoModelo, CarregarModelos);

  const acoesMotorOcr = CriarAcoesDeCatalogo(
    { baixar: BaixarMotor, remover: RemoverMotor }, setProgressoMotor, setBaixandoMotor, CarregarMotores);

  const acoesMotorVoz = CriarAcoesDeCatalogo(
    { baixar: BaixarMotorTts, remover: RemoverMotorTts }, setProgressoMotorTts, setBaixandoMotorTts, CarregarMotoresTts);

  const acoesMotorEscuta = CriarAcoesDeCatalogo(
    { baixar: BaixarMotorStt, remover: RemoverMotorStt }, setProgressoMotorStt, setBaixandoMotorStt, CarregarMotoresStt);

  const TrocarMotorOcr = (nome: string) => {
    const motor = motores.find(m => m.nome === nome);
    setTrocandoMotor(nome);
    setProgressoMotor(prev => ({ ...prev, [nome]: 'Ativando…' }));
    TrocarMotor(nome)
      .then(() => {
        // O hardware de processamento passa a depender do motor: se o novo motor não suportar o
        // hardware/API atual (ex.: trocar para um motor só-CPU com uma GPU selecionada), a config
        // migra para uma combinação suportada, avisando o usuário.
        migrarHardwareParaMotor(motor);
        setProgressoMotor(prev => ({ ...prev, [nome]: '✅ Motor ativado.' }));
        CarregarMotores();
        CarregarModelos(); // o novo motor ativo pode expor um catálogo de modelos diferente
      })
      .catch((err: any) => {
        setProgressoMotor(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
      })
      .finally(() => setTrocandoMotor(null));
  };

  // Migra o hardware/API para uma combinação que o motor recém-ativado suporte (avisando o usuário).
  const migrarHardwareParaMotor = (motor?: main.MotorOcrInfo) => {
    if (!configuracoesApp || !motor) return;

    const hwAtual = configuracoesApp.hardwareSelecionado;
    if (ehCpuNome(hwAtual)) return; // CPU é compatível com qualquer motor — nada a migrar.

    // GPU selecionada, mas o motor recém-ativado é só-CPU → processamento volta para a CPU.
    if (!motorAceleraPorGpu(motor.variante)) {
      const cpuNome = infoHardware?.cpu || HARDWARE_CPU;
      AplicarConfiguracao({ hardwareSelecionado: cpuNome, dispositivoOcr: DISPOSITIVO_CPU });
      setAvisoCompatibilidade(`O motor ${motor.rotulo} não é compatível com o hardware "${hwAtual}". O processamento foi alterado para a CPU.`);
      return;
    }

    // GPU + motor com WebGPU: normaliza a API (inclusive valores legados "directml"/"cuda").
    if (configuracoesApp.dispositivoOcr !== DISPOSITIVO_WEBGPU) {
      AplicarConfiguracao({ dispositivoOcr: DISPOSITIVO_WEBGPU });
    }
  };

  // Dispara a síntese em lote de TODAS as palavras dos dicionários para o cache de áudio, no motor
  // atualmente selecionado. O andamento chega pelo evento "tts_precache_progresso".
  const PreCarregarAudioTts = () => {
    if (!configuracoesApp) return;
    setProgressoPreCacheTts(criarProgressoPreCache('Iniciando…', true));
    PreCarregarCacheTts(configuracoesApp.motorTtsAtivo)
      .catch((err: any) => {
        setProgressoPreCacheTts(criarProgressoPreCache('⚠️ ' + String(err), false));
      });
  };

  const PararPreCarregarAudioTts = () => {
    PararPreCacheTts().catch(() => { });
  };

  // Troca o MODELO (pesos) do motor ativo. Não mexe no hardware: a compatibilidade de aceleração é do
  // MOTOR, não do peso — a migração de hardware acontece na troca de motor (migrarHardwareParaMotor).
  const trocarModelo = (novoModelo: string) => {
    AplicarConfiguracao({ modeloOcr: novoModelo });
  };

  // ----- Utilitários -----
  // Listagens best-effort: uma falha aqui deixa o catálogo vazio na aba (o usuário reabre), e o erro
  // real de download/troca aparece no progresso do item.
  function CarregarModelos() {
    ListarModelos().then(m => setModelos(m || [])).catch(() => { });
  }

  function CarregarMotores() {
    ListarMotores().then(m => setMotores(m || [])).catch(() => { });
  }

  function CarregarMotoresTts() {
    ListarMotoresTts().then(m => setMotoresTts(m || [])).catch(() => { });
  }

  function CarregarMotoresStt() {
    ListarMotoresStt().then(m => setMotoresStt(m || [])).catch(() => { });
  }

  const ehCpuNome = (hw: string): boolean => hw === HARDWARE_CPU || hw === infoHardware?.cpu;

  // Um motor acelera por GPU se a sua `variante` inclui WebGPU. WebGPU acelera em qualquer GPU
  // (Nvidia/AMD/Intel — D3D12 no Windows, Vulkan no Linux).
  const motorAceleraPorGpu = (variante: string) => (variante || HARDWARE_CPU).toLowerCase().includes(VARIANTE_WEBGPU);

  return {
    modelos, progressoModelo, baixandoModelo,
    motores, progressoMotor, baixandoMotor, trocandoMotor,
    motoresTts, progressoMotorTts, baixandoMotorTts, progressoPreCacheTts,
    motoresStt, progressoMotorStt, baixandoMotorStt,
    avisoCompatibilidade, setAvisoCompatibilidade,

    CarregarModelos,
    BaixarModeloOcr: acoesModeloOcr.baixar,
    RemoverModeloOcr: acoesModeloOcr.remover,
    trocarModelo,

    BaixarMotorOcr: acoesMotorOcr.baixar,
    RemoverMotorOcr: acoesMotorOcr.remover,
    TrocarMotorOcr,

    BaixarMotorVoz: acoesMotorVoz.baixar,
    RemoverMotorVoz: acoesMotorVoz.remover,
    BaixarMotorEscuta: acoesMotorEscuta.baixar,
    RemoverMotorEscuta: acoesMotorEscuta.remover,
    PreCarregarAudioTts,
    PararPreCarregarAudioTts,

    ehCpuNome,
  };
}
