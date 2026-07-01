import { useState, useEffect, useRef } from 'react';
import './App.css';
import { PainelConfiguracoes } from './configuracoes/PainelConfiguracoes';
import { ModalCartaoDetalhes } from './dicionario/ModalCartaoDetalhes';
import { ModalAdicionarHanzi } from './dicionario/ModalAdicionarHanzi';
import { ModalAvisoCompatibilidade } from './comum/ModalAvisoCompatibilidade';
import { ModalConfirmacao } from './comum/ModalConfirmacao';
import { AbaDescobrimento } from './descobrimento/AbaDescobrimento';
import { AbaEstudos } from './estudos/AbaEstudos';
import { config, main, progresso } from '../wailsjs/go/models';
import { CaptureAndOCR, GetConfig, SaveConfig, AddVocab, GetVocab, ShowHighlight, HideHoverPopup, ShowHoverPopup, LookupWord, DecomposeCharacter, CaractereCompleto, MarcarVistoSilencioso, GetSystemHardware, ListarModelos, BaixarModelo, RemoverModelo, ListarMotores, BaixarMotor, RemoverMotor, TrocarMotor, GetStorageInfo, LimparArmazenamento, ExcluirTudo, AbrirPastaDados, GetCaptureResolution, GetSessionImage, GetMonitores } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";

function App() {
  const [abaAtiva, setAbaAtiva] = useState('descobrimento');
  const [painelConfigAberto, setPainelConfigAberto] = useState(false);
  
  const [cartoes, setCartoes] = useState<any[]>([]); // Raw OCR result (Descobrimento)
  const [cartoesSecao, setCartoesSecao] = useState<any[]>([]); // Accumulated OCR (Palavras dessa Seção)
  const [cartaoSelecionado, setCartaoSelecionado] = useState<any | null>(null);
  const [imagemModalBase64, setImagemModalBase64] = useState<string | null>(null);
  const [dadosDecomposicao, setDadosDecomposicao] = useState<any>(null);
  
  const [cartoesVocabulario, setCartoesVocabulario] = useState<progresso.Vocab[]>([]);
  const [status, setStatus] = useState('Aguardando...');
  const [configuracoesApp, setConfiguracoesApp] = useState<config.Config | null>(null);
  const [infoHardware, setInfoHardware] = useState<main.SystemHardware | null>(null);
  const [monitores, setMonitores] = useState<any[]>([]);
  const [resCaptura, setResCaptura] = useState<main.Resolucao | null>(null);
  const [modelos, setModelos] = useState<main.ModeloOcrInfo[]>([]);
  const [progressoModelo, setProgressoModelo] = useState<Record<string, string>>({});
  const [baixandoModelo, setBaixandoModelo] = useState<string | null>(null);
  const [motores, setMotores] = useState<main.MotorOcrInfo[]>([]);
  const [progressoMotor, setProgressoMotor] = useState<Record<string, string>>({});
  const [baixandoMotor, setBaixandoMotor] = useState<string | null>(null);
  const [trocandoMotor, setTrocandoMotor] = useState<string | null>(null);
  const [avisoCompatibilidade, setAvisoCompatibilidade] = useState<string | null>(null);
  const [infoArmazenamento, setInfoArmazenamento] = useState<main.StorageInfo | null>(null);
  const [armazenamentoOcupado, setArmazenamentoOcupado] = useState(false);
  const [confirmacao, setConfirmacao] = useState<{ titulo: string; mensagem: string; rotuloAcao: string; acao: () => void } | null>(null);
  const [posicaoMouse, setPosicaoMouse] = useState({x: 0, y: 0});
  const [cartaoEmFoco, setCartaoEmFoco] = useState<any | null>(null);
  const [abaConfiguracao, setAbaConfiguracao] = useState('Geral');
  const [termoBusca, setTermoBusca] = useState('');
  const [totalHanzis, setTotalHanzis] = useState<number>(0);
  const [modalAdicionarHanzi, setModalAdicionarHanzi] = useState<{ open: boolean, status: string }>({ open: false, status: '' });
  const [inputAdicionarHanzi, setInputAdicionarHanzi] = useState('');
  const [sugestoesPinyin, setSugestoesPinyin] = useState<string[]>([]);
  
  const cartoesRef = useRef<any[]>([]);
  const cartaoEmFocoRef = useRef<any | null>(null);
  const configuracoesAppRef = useRef<config.Config | null>(null);
  const abaAtivaRef = useRef<string>('descobrimento');
  const timeoutPopupRef = useRef<any>(null);
  const ultimaPosicaoMouseRef = useRef<{x: number, y: number}>({x: 0, y: 0});
  const offsetMonitorRef = useRef<{x: number, y: number}>({x: 0, y: 0});
  const mouseSobreCartaoUIRef = useRef<boolean>(false);

  useEffect(() => {
    cartoesRef.current = cartoes;
  }, [cartoes]);

  useEffect(() => {
    if (cartaoSelecionado && cartaoSelecionado.imageId) {
      GetSessionImage(cartaoSelecionado.imageId).then(base64 => {
        setImagemModalBase64(base64);
      });
    } else {
      setImagemModalBase64(null);
    }
  }, [cartaoSelecionado]);

  useEffect(() => {
    cartaoEmFocoRef.current = cartaoEmFoco;
  }, [cartaoEmFoco]);

  useEffect(() => {
    configuracoesAppRef.current = configuracoesApp;
  }, [configuracoesApp]);

  useEffect(() => {
    abaAtivaRef.current = abaAtiva;
  }, [abaAtiva]);

  useEffect(() => {
    GetConfig().then(cfg => setConfiguracoesApp(cfg));
    GetSystemHardware().then(hw => setInfoHardware(hw));
    GetMonitores().then(m => {
      setMonitores(m || []);
      // Atualizar o offset do monitor alvo
      GetConfig().then(cfg => {
        if (cfg && m && m.length > 0) {
          const alvo = m.find((mon: any) => mon.id === (cfg.monitorAlvo || 0)) || m[0];
          offsetMonitorRef.current = { x: alvo?.x || 0, y: alvo?.y || 0 };
        }
      });
    });
    GetCaptureResolution().then(res => setResCaptura(res));
    CarregarModelos();
    CarregarMotores();
    CarregarVocabulario();

    // @ts-ignore
    window.go.main.App.ObterTotalHanzisDicionario().then(setTotalHanzis).catch(console.error);

    EventsOn("modelo_download_progresso", (data: any) => {
      if (!data?.nome) return;
      if (data.mensagem) {
        setProgressoModelo(prev => ({ ...prev, [data.nome]: data.mensagem }));
      } else if (data.erro) {
        setProgressoModelo(prev => ({ ...prev, [data.nome]: '⚠️ ' + data.erro }));
      }
    });

    // Motores (sidecars): progresso de download/instalação e refresh do estado ativo (bootstrap/troca).
    EventsOn("motor_download_progresso", (data: any) => {
      if (!data?.nome) return;
      if (data.mensagem) {
        setProgressoMotor(prev => ({ ...prev, [data.nome]: data.mensagem }));
      } else if (data.erro) {
        setProgressoMotor(prev => ({ ...prev, [data.nome]: '⚠️ ' + data.erro }));
      }
    });
    EventsOn("ocr_pronto", () => { CarregarMotores(); });
    EventsOn("motor_bootstrap_fim", () => { CarregarMotores(); });

    EventsOn("trigger_scan", () => {
      EscanearTelaEhProcessar();
    });

    EventsOn("trigger_save", () => {
      if (cartaoEmFocoRef.current) {
        SalvarPalavra(cartaoEmFocoRef.current, 'estudo');
      }
    });

    EventsOn("mouse_pos", (data: any) => {
      // Se o usuário está interagindo com os cartoes na interface, o rastreador global não deve interferir
      if (mouseSobreCartaoUIRef.current) return;
      
      setPosicaoMouse({x: data.x, y: data.y});
      
      // Converter coordenadas globais do mouse para coordenadas locais do monitor alvo
      const localX = data.x - offsetMonitorRef.current.x;
      const localY = data.y - offsetMonitorRef.current.y;
      
      // Encontrar a caixa mais próxima do mouse (ao invés de exigir colisão estrita)
      let found: any = null;
      let minDistance = Infinity;

      if (abaAtivaRef.current === 'descobrimento') {
        const maxDist = configuracoesAppRef.current?.distanciaMaximaHoverPx || 220;
        
        for (const c of cartoesRef.current) {
          if (c.caixa && c.caixa.length === 4) {
            const [x0, y0, x1, y1] = c.caixa;
            
            // Distância de um ponto a um retângulo (usando coordenadas locais)
            let dx = 0;
            if (localX < x0) dx = x0 - localX;
            else if (localX > x1) dx = localX - x1;

            let dy = 0;
            if (localY < y0) dy = y0 - localY;
            else if (localY > y1) dy = localY - y1;

            const dist = Math.sqrt(dx * dx + dy * dy);

            if (dist < minDistance && dist <= maxDist) {
              minDistance = dist;
              found = c;
            }
          }
        }
      }
      
      if (found) {
        setCartaoEmFoco(found);
        
        // Lógica do Pop-up Estacionário
        if (configuracoesAppRef.current?.habilitarPopupHover) {
          const dx = data.x - ultimaPosicaoMouseRef.current.x;
          const dy = data.y - ultimaPosicaoMouseRef.current.y;
          const moveDist = Math.sqrt(dx * dx + dy * dy);

          // Se moveu mais de 5 pixels, consideramos movimento ativo
          if (moveDist > 5) {
            ultimaPosicaoMouseRef.current = { x: data.x, y: data.y };
            
            // Cancela o agendamento anterior
            if (timeoutPopupRef.current) {
              clearTimeout(timeoutPopupRef.current);
            }
            
            // Oculta o popup enquanto está em movimento (opcional)
            HideHoverPopup();

            // Agenda um novo popup para quando estacionar
            const delay = configuracoesAppRef.current?.tempoParadoPopupMs || 500;
            timeoutPopupRef.current = setTimeout(() => {
              ShowHoverPopup(found.pinyin || '', found.hanzi || '', found.significados ? found.significados.join(', ') : '', data.x, data.y);
            }, delay);
          }
        }
      } else {
        if (cartaoEmFocoRef.current != null) {
          setCartaoEmFoco(null);
          HideHoverPopup();
          if (timeoutPopupRef.current) {
            clearTimeout(timeoutPopupRef.current);
          }
        }
      }
    });

  }, []);

  useEffect(() => {
    EventsOn("toggle_popup_hover", () => {
      if (configuracoesAppRef.current) {
        const newValue = !configuracoesAppRef.current.habilitarPopupHover;
        const novo = { ...configuracoesAppRef.current, habilitarPopupHover: newValue } as config.Config;
        setConfiguracoesApp(novo);
        SaveConfig(novo);
      }
    });
  }, []);

  // Carrega o uso de disco ao abrir a aba Armazenamento (ou ao pesquisar nas configurações).
  useEffect(() => {
    if (painelConfigAberto && (abaConfiguracao === 'Armazenamento' || termoBusca)) {
      CarregarArmazenamento();
    }
  }, [painelConfigAberto, abaConfiguracao, termoBusca]);

  // Efeito para destacar palavras "Em estudo" na tela
  useEffect(() => {
    // @ts-ignore
    if (!window.go || !window.go.main || !window.go.main.App.ShowEstudoHighlights) return;

    if (!configuracoesApp?.destacarEstudoTela) {
      // @ts-ignore
      window.go.main.App.ShowEstudoHighlights([]);
      return;
    }

    const cardsAtuais = abaAtiva === 'descobrimento' ? cartoes : (abaAtiva === 'tela_unica' ? cartoesSecao : []);
    if (cardsAtuais.length === 0) {
      // @ts-ignore
      window.go.main.App.ShowEstudoHighlights([]);
      return;
    }

    const estudoWords = new Set(
      cartoesVocabulario.filter(v => v.Status === 'estudo').map(v => v.Hanzi)
    );

    const boxes: number[][] = [];
    for (const c of cardsAtuais) {
      const hz = c.hanzi || c.Hanzi;
      if (hz && estudoWords.has(hz) && c.caixa && c.caixa.length === 4) {
        boxes.push(c.caixa);
      }
    }
    // @ts-ignore
    window.go.main.App.ShowEstudoHighlights(boxes);

  }, [configuracoesApp?.destacarEstudoTela, abaAtiva, cartoes, cartoesSecao, cartoesVocabulario]);

  const CarregarVocabulario = () => {
    GetVocab().then(v => setCartoesVocabulario(v || []));
  };

  const CarregarModelos = () => {
    ListarModelos().then(m => setModelos(m || [])).catch(() => {});
  };

  const FormatarTamanho = (bytes: number): string => {
    if (!bytes) return '';
    const mb = bytes / (1024 * 1024);
    if (mb >= 1024) return (mb / 1024).toFixed(1) + ' GB';
    return mb.toFixed(1) + ' MB';
  };

  const BaixarModeloOcr = (nome: string) => {
    setBaixandoModelo(nome);
    setProgressoModelo(prev => ({ ...prev, [nome]: 'Iniciando download…' }));
    BaixarModelo(nome)
      .then(() => {
        setProgressoModelo(prev => ({ ...prev, [nome]: '✅ Instalado!' }));
        CarregarModelos();
      })
      .catch((err: any) => {
        setProgressoModelo(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
      })
      .finally(() => setBaixandoModelo(null));
  };

  const RemoverModeloOcr = (nome: string) => {
    RemoverModelo(nome)
      .then(() => {
        setProgressoModelo(prev => {
          const copia = { ...prev };
          delete copia[nome];
          return copia;
        });
        CarregarModelos();
      })
      .catch((err: any) => {
        setProgressoModelo(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
      });
  };

  const CarregarMotores = () => {
    ListarMotores().then(m => setMotores(m || [])).catch(() => {});
  };

  const BaixarMotorOcr = (nome: string) => {
    setBaixandoMotor(nome);
    setProgressoMotor(prev => ({ ...prev, [nome]: 'Iniciando download…' }));
    BaixarMotor(nome)
      .then(() => {
        setProgressoMotor(prev => ({ ...prev, [nome]: '✅ Instalado!' }));
        CarregarMotores();
      })
      .catch((err: any) => {
        setProgressoMotor(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
      })
      .finally(() => setBaixandoMotor(null));
  };

  const RemoverMotorOcr = (nome: string) => {
    RemoverMotor(nome)
      .then(() => {
        setProgressoMotor(prev => {
          const copia = { ...prev };
          delete copia[nome];
          return copia;
        });
        CarregarMotores();
      })
      .catch((err: any) => {
        setProgressoMotor(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
      });
  };

  const TrocarMotorOcr = (nome: string) => {
    setTrocandoMotor(nome);
    setProgressoMotor(prev => ({ ...prev, [nome]: 'Ativando…' }));
    TrocarMotor(nome)
      .then(() => {
        setProgressoMotor(prev => {
          const copia = { ...prev };
          delete copia[nome];
          return copia;
        });
        CarregarMotores();
      })
      .catch((err: any) => {
        setProgressoMotor(prev => ({ ...prev, [nome]: '⚠️ ' + String(err) }));
      })
      .finally(() => setTrocandoMotor(null));
  };

  const CarregarArmazenamento = () => {
    GetStorageInfo().then(info => setInfoArmazenamento(info)).catch(() => {});
  };

  const LimparCategoriaArmazenamento = (chave: string) => {
    setArmazenamentoOcupado(true);
    LimparArmazenamento(chave)
      .then(() => CarregarArmazenamento())
      .catch((err: any) => setStatus('⚠️ ' + String(err)))
      .finally(() => setArmazenamentoOcupado(false));
  };

  const ExcluirTodoArmazenamento = () => {
    setArmazenamentoOcupado(true);
    ExcluirTudo()
      .then(() => {
        CarregarArmazenamento();
        CarregarModelos();
        CarregarVocabulario();
        setStatus('Armazenamento limpo.');
      })
      .catch((err: any) => setStatus('⚠️ ' + String(err)))
      .finally(() => setArmazenamentoOcupado(false));
  };

  const AtualizarConfiguracao = (key: keyof config.Config, value: any) => {
    if (!configuracoesApp) return;
    
    let mudancas: Partial<config.Config> = { [key]: value };

    // Atualizar offset do monitor quando o alvo mudar
    if (key === 'monitorAlvo' && monitores.length > 0) {
      const newMon = monitores.find((mon: any) => mon.id === value) || monitores[0];
      if (newMon) {
        offsetMonitorRef.current = { x: newMon.x || 0, y: newMon.y || 0 };
      }
    }

    AplicarConfiguracao(mudancas);
  };

  // Aplica várias mudanças de config de uma só vez, sobre o mesmo snapshot.
  // Necessário porque chamadas encadeadas de AtualizarConfiguracao usariam o
  // configuracoesApp "velho" do closure e uma sobrescreveria a outra ao salvar.
  const AplicarConfiguracao = (mudancas: Partial<config.Config>) => {
    if (!configuracoesApp) return;
    const novo = { ...configuracoesApp, ...mudancas } as config.Config;
    setConfiguracoesApp(novo);
    SaveConfig(novo);
  };

  // ----- Matriz de compatibilidade OCR x Hardware x API -----
  // Regra (invertida): o MODELO é a escolha primária e nunca é bloqueado. O hardware/API é que
  // ficam bloqueados quando o modelo atual não os suporta. Ao trocar para um modelo incompatível
  // com o hardware atual, a config migra automaticamente para uma suportada (com aviso em pop-up).
  const ehNvidia = (hw: string) => (hw || '').toLowerCase().includes('nvidia');

  const ehCpuNome = (hw: string): boolean => hw === 'CPU' || hw === infoHardware?.cpu;

  const hardwareEhCpu = (): boolean => {
    if (!configuracoesApp) return true;
    return ehCpuNome(configuracoesApp.hardwareSelecionado);
  };

  const rotuloModelo = (m: string): string => (m === 'EasyOCR (Download)' ? 'EasyOCR' : m);

  // EasyOCR em GPU exige Nvidia (CUDA); não roda em GPU não-Nvidia. RapidOCR roda em qualquer GPU.
  const hardwareCompativelComModelo = (modelo: string, hardwareNome: string): boolean => {
    if (ehCpuNome(hardwareNome)) return true;
    if (modelo === 'EasyOCR (Download)') return ehNvidia(hardwareNome);
    return true;
  };

  // EasyOCR não suporta DirectML; CPU e CUDA são aceitos por todos.
  const apiCompativelComModelo = (modelo: string, api: string): boolean => {
    if (api === 'directml') return modelo !== 'EasyOCR (Download)';
    return true;
  };

  // Troca o modelo e, se o hardware/API atual for incompatível, migra para uma config suportada,
  // avisando o usuário pelo pop-up.
  const trocarModelo = (novoModelo: string) => {
    if (!configuracoesApp) return;

    const mudancas: Partial<config.Config> = { modeloOcr: novoModelo };
    const hwAtual = configuracoesApp.hardwareSelecionado;
    const cpuNome = infoHardware?.cpu || 'CPU';
    let aviso: string | null = null;

    if (!ehCpuNome(hwAtual) && !hardwareCompativelComModelo(novoModelo, hwAtual)) {
      // GPU incompatível (ex.: EasyOCR numa GPU não-Nvidia) → cai para CPU
      mudancas.hardwareSelecionado = cpuNome;
      mudancas.dispositivoOcr = 'cpu';
      aviso = `O hardware "${hwAtual}" não é compatível com ${rotuloModelo(novoModelo)}, que em GPU exige uma placa Nvidia (CUDA). O processamento foi alterado para a CPU.`;
    } else if (!ehCpuNome(hwAtual) && !apiCompativelComModelo(novoModelo, configuracoesApp.dispositivoOcr)) {
      // API incompatível (ex.: EasyOCR + DirectML numa GPU Nvidia) → CUDA se Nvidia, senão CPU
      if (ehNvidia(hwAtual)) {
        mudancas.dispositivoOcr = 'cuda';
        aviso = `A API "DirectML" não é suportada por ${rotuloModelo(novoModelo)}. A API foi alterada para CUDA.`;
      } else {
        mudancas.hardwareSelecionado = cpuNome;
        mudancas.dispositivoOcr = 'cpu';
        aviso = `${rotuloModelo(novoModelo)} não suporta a configuração de GPU atual. O processamento foi alterado para a CPU.`;
      }
    }

    AplicarConfiguracao(mudancas);
    if (aviso) setAvisoCompatibilidade(aviso);
  };

  const SalvarPalavra = (c: any, newStatus: string) => {
    const sig = c.significados ? c.significados.join(', ') : c.Significado || '';
    const py = c.pinyin || c.Pinyin || '';
    const hz = c.hanzi || c.Hanzi;
    
    AddVocab(hz, py, sig, newStatus).then(() => {
      CarregarVocabulario();
      // NOTA: O usuário expressou preferência por manter os cartoes na seção em vez de "inbox zero",
      // usando um tratamento visual (cor + sort) para diferenciá-los, então não faremos setCartoes(filter).
      setStatus(`Palavra movida para ${newStatus}: ${hz}`);
    });
  };

  const EscanearTelaEhProcessar = () => {
    setStatus('Capturando e processando OCR...');
    CaptureAndOCR()
      .then((res: any) => {
        setStatus('Captura concluída!');
        const newCards = res || [];
        setCartoes(newCards);
        
        // Acumula os cartoes na "Seção", permitindo que o usuário tire várias fotos e explore todas
        const secaoClean = newCards.map((c: any) => ({ ...c, caixa: [] }));
        setCartoesSecao(prev => [...prev, ...secaoClean]);
        
        CarregarVocabulario(); // Reload vocab para status refletir na UI imediatamente
      })
      .catch((err: any) => {
        let msg = String(err);
        try {
          const match = msg.match(/API error \(\d+\): (\{.*\})/);
          if (match && match[1]) {
            const parsed = JSON.parse(match[1]);
            if (parsed.error) msg = parsed.error;
          }
        } catch(e) {}
        setStatus('⚠️ ' + msg);
      });
  };

  const AoEntrarNoCartao = (c: any) => {
    mouseSobreCartaoUIRef.current = true;
    if (c.caixa && c.caixa.length === 4) {
      ShowHighlight(
        Math.round(c.caixa[0]),
        Math.round(c.caixa[1]),
        Math.round(c.caixa[2]),
        Math.round(c.caixa[3])
      );
    }
  };

  const AoSairDoCartao = () => {
    mouseSobreCartaoUIRef.current = false;
    HideHoverPopup();
  };

  const AoClicarNoCartao = (c: any) => {
    setCartaoSelecionado(c);
    setDadosDecomposicao(null);
    const hz = c.hanzi || c.Hanzi;
    
    if (hz.length > 1) {
      // Multi-character: decompose into individual characters
      const chars = Array.from(hz);
      Promise.all(chars.map(char => LookupWord(char as string).then(res => ({ char, res }))))
        .then(results => {
          setDadosDecomposicao({ type: 'multi', data: results });
        });
    } else if (hz.length === 1) {
      // Single character: decompose into radicals using MakeMeAHanzi
      DecomposeCharacter(hz).then(res => {
        setDadosDecomposicao({ type: 'single', data: res });
      });
    }
  };

  const AoClicarNoCaractereDecomposto = async (char: string) => {
    // Se for abreviação visual, redireciona para o caractere completo
    const completo = await CaractereCompleto(char);
    const charFinal = completo || char;
    const foiAbreviacao = completo !== '';

    LookupWord(charFinal).then(entradas => {
      if (entradas && entradas.length > 0) {
        const ent = entradas[0];
        const newCard = {
          hanzi: ent.Simplificado,
          Hanzi: ent.Simplificado,
          pinyin: ent.Pinyin,
          significados: ent.Significados
        };
        AoClicarNoCartao(newCard);
        
        // Abreviações visuais não entram para o banco de dados
        if (!foiAbreviacao) {
          MarcarVistoSilencioso(charFinal).then(() => CarregarVocabulario());
        }
      }
    });
  };

  const DeduplicarCartoes = (rawCards: any[]) => {
    const map = new Map<string, any>();
    rawCards.forEach(c => {
      if (!map.has(c.hanzi)) {
        map.set(c.hanzi, c);
      }
    });
    return Array.from(map.values());
  };

  // Obter listas
  const vistas = cartoesVocabulario; // Tudo que está no banco foi "visto"
  const estudando = cartoesVocabulario.filter(c => c.Status === 'estudo');
  const aprendidas = cartoesVocabulario.filter(c => c.Status === 'aprendido');

  const RenderizarListaCartoes = (list: any[], defaultStatus: string, actionBtns: (c: any) => JSX.Element) => {
    if (list.length === 0) {
      return <div style={{ color: 'var(--cor-texto-suave)', textAlign: 'center', marginTop: '20px' }}>Nenhuma palavra encontrada.</div>;
    }
    
    // As tags não interferem no sort, mantém a ordem original da lista
    const sortedList = list;

    return (
      <div className="cartoes-container">
        {sortedList.map((c, i) => {
          const hz = c.hanzi || c.Hanzi;
          const py = c.pinyin || c.Pinyin || '---';
          const sigs = c.significados ? c.significados.join(', ') : c.Significado || 'Sem tradução';
          
          const statusDB = cartoesVocabulario.find(v => v.Hanzi === hz)?.Status || defaultStatus;
          
          // Destaque para palavras estudadas/aprendidas
          const cardStyle: React.CSSProperties = {};
          let badge = null;
          
          if (statusDB === 'estudo') {
            cardStyle.borderColor = '#2196f3';
            cardStyle.backgroundColor = '#1a2733';
            badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#2196f3', fontWeight: 'bold'}}>ESTUDO</div>;
          } else if (statusDB === 'aprendido') {
            cardStyle.borderColor = '#4caf50';
            cardStyle.backgroundColor = '#1e2e1e';
            badge = <div style={{position: 'absolute', top: '4px', right: '4px', fontSize: '9px', color: '#4caf50', fontWeight: 'bold'}}>APRENDIDA</div>;
          }

          return (
            <div 
              className="card" 
              key={i}
              style={{...cardStyle, position: 'relative'}}
              onMouseEnter={() => AoEntrarNoCartao(c)}
              onMouseLeave={AoSairDoCartao}
              onClick={() => AoClicarNoCartao(c)}
            >
              {badge}
              <div className="card-pinyin" style={{color: statusDB === 'estudo' ? '#64b5f6' : statusDB === 'aprendido' ? '#81c784' : 'var(--cor-destaque)'}}>{py}</div>
              <div className="card-hanzi">{hz}</div>
              <div className="card-sigs">
                {sigs}
              </div>
              <div className="card-actions" onClick={(e) => e.stopPropagation()}>
                {actionBtns(c)}
              </div>
            </div>
          )
        })}
      </div>
    );
  };

  const ObterTituloJanela = () => {
    switch (abaAtiva) {
      case 'descobrimento': return 'Descobrimento (Último OCR)';
      case 'tela_unica': return 'Palavras Dessa Seção (Acumulado)';
      case 'vistas': return 'Histórico: Já Vistas';
      case 'estudando': return 'Estudando';
      case 'aprendidas': return 'Vocabulário (Aprendidas)';
      default: return '';
    }
  };

  return (
    <div id="App">
      {/* Sidebar Navigation */}
      <div className="sidebar">
        <h1>Chinese Study</h1>
        
        <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', margin: '10px 0 5px 10px', textTransform: 'uppercase', fontWeight: 'bold' }}>Sessão Atual</div>
        <button 
          className={`sidebar-btn ${abaAtiva === 'descobrimento' ? 'active' : ''}`}
          onClick={() => setAbaAtiva('descobrimento')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path></svg>
          Descobrimento
        </button>

        <button 
          className={`sidebar-btn ${abaAtiva === 'tela_unica' ? 'active' : ''}`}
          onClick={() => setAbaAtiva('tela_unica')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>
          Palavras Dessa Seção
        </button>

        <div style={{ fontSize: '11px', color: 'var(--cor-texto-suave)', margin: '20px 0 5px 10px', textTransform: 'uppercase', fontWeight: 'bold' }}>Banco de Dados</div>
        <button 
          className={`sidebar-btn ${abaAtiva === 'vistas' ? 'active' : ''}`}
          onClick={() => setAbaAtiva('vistas')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path><circle cx="12" cy="12" r="3"></circle></svg>
          Já Vistas
        </button>

        <button 
          className={`sidebar-btn ${abaAtiva === 'estudando' ? 'active' : ''}`}
          onClick={() => setAbaAtiva('estudando')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>
          Estudando
        </button>

        <button 
          className={`sidebar-btn ${abaAtiva === 'aprendidas' ? 'active' : ''}`}
          onClick={() => setAbaAtiva('aprendidas')}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>
          Vocabulário
        </button>

        <div className="sidebar-spacer"></div>

        {/* Display do Hover Card na Sidebar */}
        <div style={{
          backgroundColor: 'var(--cor-fundo-secundario)',
          padding: '12px',
          borderRadius: '8px',
          marginBottom: '16px',
          border: '1px solid var(--cor-borda)',
          minHeight: '120px',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          textAlign: 'center'
        }}>
          {cartaoEmFoco ? (
            <>
              <div style={{color: 'var(--cor-destaque)', fontSize: '12px'}}>{cartaoEmFoco.pinyin}</div>
              <div style={{fontSize: '28px', fontWeight: 'bold', margin: '4px 0'}}>{cartaoEmFoco.hanzi}</div>
              <div style={{fontSize: '11px', color: 'var(--cor-texto-suave)'}}>
                {cartaoEmFoco.significados ? cartaoEmFoco.significados.join(', ') : 'Sem tradução'}
              </div>
            </>
          ) : (
            <div style={{color: 'var(--cor-texto-suave)', fontSize: '12px'}}>
              Passe o mouse sobre um texto chinês para focar
            </div>
          )}
        </div>

        <button 
          className="sidebar-btn"
          onClick={() => setPainelConfigAberto(true)}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>
          Configurações
        </button>
      </div>

      {/* Main Content Area */}
      <div className="main-content">
        <div className="header">
          <div className="header-title">
            {ObterTituloJanela()}
          </div>
          
          <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
            {abaAtiva === 'descobrimento' && (
              <button className="scan-btn" onClick={EscanearTelaEhProcessar}>
                Escanear Tela ({configuracoesApp?.atalhoEscanear || 'F4'})
              </button>
            )}

            {abaAtiva === 'tela_unica' && (
              <button 
                className="scan-btn" 
                style={{ backgroundColor: '#f44336', display: 'flex', alignItems: 'center', gap: '6px' }}
                onClick={() => {
                  setCartoes([]);
                  setCartoesSecao([]);
                }}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                Limpar Seção
              </button>
            )}

            {(abaAtiva === 'estudando' || abaAtiva === 'aprendidas') && (
              <button 
                className="scan-btn" 
                style={{ backgroundColor: '#2196f3', display: 'flex', alignItems: 'center', gap: '6px' }}
                onClick={() => setModalAdicionarHanzi({ open: true, status: abaAtiva === 'estudando' ? 'estudo' : 'aprendido' })}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
                Adicionar Hanzi
              </button>
            )}

            {abaAtiva === 'vistas' && (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', fontSize: '12px', color: 'var(--cor-texto-suave)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{ width: '120px', height: '8px', backgroundColor: 'var(--cor-fundo-secundario)', borderRadius: '4px', overflow: 'hidden' }}>
                    <div style={{ width: `${totalHanzis > 0 ? (vistas.length / totalHanzis) * 100 : 0}%`, height: '100%', backgroundColor: 'var(--cor-destaque)' }}></div>
                  </div>
                  <strong style={{ color: 'var(--cor-texto-primario)' }}>
                    {totalHanzis > 0 ? ((vistas.length / totalHanzis) * 100).toFixed(2) : 0}%
                  </strong>
                </div>
                <span style={{ fontSize: '10px', marginTop: '2px' }}>{vistas.length} / {totalHanzis} hanzis descobertos</span>
              </div>
            )}
          </div>
        </div>

        <div style={{ color: 'var(--cor-texto-suave)', marginBottom: '24px' }}>
          {status}
        </div>

        <AbaDescobrimento
          abaAtiva={abaAtiva}
          cartoes={cartoes}
          cartoesSecao={cartoesSecao}
          vistas={vistas}
          cartoesVocabulario={cartoesVocabulario}
          AoEntrarNoCartao={AoEntrarNoCartao}
          AoSairDoCartao={AoSairDoCartao}
          AoClicarNoCartao={AoClicarNoCartao}
          SalvarPalavra={SalvarPalavra}
          DeduplicarCartoes={DeduplicarCartoes}
        />

        <AbaEstudos
          abaAtiva={abaAtiva}
          estudando={estudando}
          aprendidas={aprendidas}
          cartoesVocabulario={cartoesVocabulario}
          AoEntrarNoCartao={AoEntrarNoCartao}
          AoSairDoCartao={AoSairDoCartao}
          AoClicarNoCartao={AoClicarNoCartao}
          SalvarPalavra={SalvarPalavra}
        />
      </div>

            {/* Settings Modal Overlay */}
      <PainelConfiguracoes 
        painelConfigAberto={painelConfigAberto}
        setPainelConfigAberto={setPainelConfigAberto}
        configuracoesApp={configuracoesApp!}
        AtualizarConfiguracao={AtualizarConfiguracao}
        AplicarConfiguracao={AplicarConfiguracao}
        setConfirmacao={setConfirmacao}
        abaConfiguracao={abaConfiguracao}
        setAbaConfiguracao={setAbaConfiguracao}
        termoBusca={termoBusca}
        setTermoBusca={setTermoBusca}
        infoHardware={infoHardware}
        resCaptura={resCaptura}
        monitores={monitores}
        modelos={modelos}
        progressoModelo={progressoModelo}
        baixandoModelo={baixandoModelo}
        avisoCompatibilidade={avisoCompatibilidade}
        infoArmazenamento={infoArmazenamento}
        armazenamentoOcupado={armazenamentoOcupado}
        BaixarModeloOcr={BaixarModeloOcr}
        RemoverModeloOcr={RemoverModeloOcr}
        trocarModelo={trocarModelo}
        motores={motores}
        progressoMotor={progressoMotor}
        baixandoMotor={baixandoMotor}
        trocandoMotor={trocandoMotor}
        BaixarMotorOcr={BaixarMotorOcr}
        RemoverMotorOcr={RemoverMotorOcr}
        TrocarMotorOcr={TrocarMotorOcr}
        CarregarArmazenamento={CarregarArmazenamento}
        LimparCategoriaArmazenamento={LimparCategoriaArmazenamento}
        ExcluirTodoArmazenamento={ExcluirTodoArmazenamento}
        hardwareEhCpu={hardwareEhCpu}
        ehCpuNome={ehCpuNome}
        ehNvidia={ehNvidia}
        apiCompativelComModelo={apiCompativelComModelo}
        hardwareCompativelComModelo={hardwareCompativelComModelo}
        rotuloModelo={rotuloModelo}
      />

      {/* Card Details Modal Overlay */}
      <ModalCartaoDetalhes
        cartaoSelecionado={cartaoSelecionado}
        setCartaoSelecionado={setCartaoSelecionado}
        imagemModalBase64={imagemModalBase64}
        dadosDecomposicao={dadosDecomposicao}
        AoClicarNoCaractereDecomposto={AoClicarNoCaractereDecomposto}
      />
      {cartaoSelecionado && (
        <div className="modal-overlay" onClick={() => setCartaoSelecionado(null)}>
          <div className="modal-content hanzi-modal-content" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Detalhes</h2>
              <button className="modal-close" onClick={() => setCartaoSelecionado(null)}>×</button>
            </div>
            <div className="modal-body" style={{display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '20px'}}>
              
              {imagemModalBase64 && (
                <div style={{border: '1px solid var(--cor-borda)', padding: '4px', borderRadius: '4px', backgroundColor: 'var(--cor-fundo-cartao)'}}>
                  <img src={"data:image/png;base64," + imagemModalBase64} alt="Recorte" style={{maxWidth: '100%', maxHeight: '150px'}} />
                </div>
              )}

              <div style={{textAlign: 'center'}}>
                <div style={{color: 'var(--cor-pinyin)', fontSize: '24px'}}>{cartaoSelecionado.pinyin || cartaoSelecionado.Pinyin}</div>
                <div style={{fontFamily: 'var(--fonte-hanzi)', fontSize: '64px', fontWeight: 'bold', lineHeight: '1.2'}}>{cartaoSelecionado.hanzi || cartaoSelecionado.Hanzi}</div>
                <div style={{color: 'var(--cor-texto-suave)', fontSize: '18px', marginTop: '10px'}}>{cartaoSelecionado.significados ? cartaoSelecionado.significados.join(', ') : cartaoSelecionado.Significado}</div>
              </div>

              {dadosDecomposicao && (
                <div style={{width: '100%', borderTop: '1px solid var(--cor-borda)', paddingTop: '20px'}}>
                  <h3 style={{fontSize: '16px', color: 'var(--cor-texto-primario)', marginBottom: '16px'}}>Decomposição</h3>
                  
                  {dadosDecomposicao.type === 'single' ? (
                    <div style={{backgroundColor: 'var(--cor-fundo-cartao)', padding: '16px', borderRadius: '8px'}}>
                      {dadosDecomposicao.data?.pinyin && dadosDecomposicao.data.pinyin.length > 0 && (
                        <div style={{fontSize: '14px', marginBottom: '8px'}}>
                          <strong>Pinyin (MakeMeAHanzi):</strong> {dadosDecomposicao.data.pinyin.join(', ')}
                        </div>
                      )}
                      {dadosDecomposicao.data?.definition && (
                        <div style={{fontSize: '14px', marginBottom: '8px'}}>
                          <strong>Definição:</strong> {dadosDecomposicao.data.definition}
                        </div>
                      )}
                      <div style={{fontSize: '14px'}}>
                        <strong>Radical:</strong> {dadosDecomposicao.data?.radical || '-'}
                      </div>
                      {dadosDecomposicao.data?.abreviacoes && dadosDecomposicao.data.abreviacoes.length > 0 && (
                        <div style={{fontSize: '14px', marginTop: '8px'}}>
                          <strong>Abreviações visuais:</strong>
                          <span style={{display: 'inline-flex', gap: '6px', marginLeft: '8px', alignItems: 'center'}}>
                            {dadosDecomposicao.data.abreviacoes.map((a: string, i: number) => (
                              <span
                                key={i}
                                style={{
                                  fontFamily: 'var(--fonte-hanzi)',
                                  fontSize: '22px',
                                  backgroundColor: 'var(--cor-fundo-entrada)',
                                  border: '1px solid var(--cor-borda)',
                                  borderRadius: '6px',
                                  padding: '2px 8px',
                                }}
                              >{a}</span>
                            ))}
                          </span>
                        </div>
                      )}
                      <div style={{fontSize: '14px', marginTop: '8px'}}>
                        <strong>Estrutura:</strong> {(() => {
                          const raw = dadosDecomposicao.data?.decomposition || '';
                          if (!raw) return '-';

                          const mapaIdc: Record<string, string> = {
                            '⿰': 'Esquerda–Direita',
                            '⿱': 'Cima–Baixo',
                            '⿲': 'Esquerda–Centro–Direita',
                            '⿳': 'Cima–Centro–Baixo',
                            '⿴': 'Cercado',
                            '⿵': 'Aberto embaixo',
                            '⿶': 'Aberto em cima',
                            '⿷': 'Aberto à direita',
                            '⿸': 'Cobertura superior-esquerda',
                            '⿹': 'Cobertura superior-direita',
                            '⿺': 'Cobertura inferior-esquerda',
                            '⿻': 'Sobreposto',
                          };

                          const chars: string[] = Array.from(raw);
                          const estrutura = mapaIdc[chars[0]] || null;
                          const componentes = chars.filter((c: string) => !mapaIdc[c] && c !== '?');

                          return estrutura
                            ? `${chars[0]} ${estrutura}`
                            : raw;
                        })()}
                      </div>
                      {dadosDecomposicao.data?.etymology && Object.keys(dadosDecomposicao.data.etymology).length > 0 && (
                        <div style={{fontSize: '14px', marginTop: '8px'}}>
                          <strong>Etimologia:</strong> {(() => {
                            const e = dadosDecomposicao.data.etymology;
                            const tMap: Record<string, string> = {
                              'pictophonetic': 'Pictofonético',
                              'ideographic': 'Ideográfico',
                              'pictographic': 'Pictográfico',
                            };
                            let txt = tMap[e.type] || e.type || 'Desconhecida';
                            if (e.hint) txt += ` — ${e.hint}`;
                            if (e.semantic) txt += ` (Semântica: ${e.semantic})`;
                            if (e.phonetic) txt += ` (Fonética: ${e.phonetic})`;
                            return txt;
                          })()}
                        </div>
                      )}
                      {(() => {
                        const raw = dadosDecomposicao.data?.decomposition || '';
                        if (!raw) return null;

                        const mapaIdc: Record<string, string> = {
                          '⿰': '', '⿱': '', '⿲': '', '⿳': '', '⿴': '', '⿵': '',
                          '⿶': '', '⿷': '', '⿸': '', '⿹': '', '⿺': '', '⿻': '',
                        };

                        const componentes: string[] = (Array.from(raw) as string[]).filter((c: string) => !(c in mapaIdc) && c !== '?');

                        if (componentes.length === 0) return null;

                        return (
                          <div style={{marginTop: '12px'}}>
                            <strong style={{fontSize: '14px'}}>Componentes:</strong>
                            <div style={{display: 'flex', gap: '8px', marginTop: '8px', flexWrap: 'wrap'}}>
                              {componentes.map((comp, idx) => (
                                <div
                                  key={idx}
                                  style={{
                                    fontSize: '28px',
                                    fontFamily: 'var(--fonte-hanzi)',
                                    backgroundColor: 'var(--cor-fundo-entrada)',
                                    border: '1px solid var(--cor-borda)',
                                    borderRadius: '8px',
                                    padding: '8px 14px',
                                    cursor: 'pointer',
                                    transition: 'all 0.2s',
                                  }}
                                  onClick={() => AoClicarNoCaractereDecomposto(comp)}
                                >
                                  {comp}
                                </div>
                              ))}
                            </div>
                          </div>
                        );
                      })()}
                    </div>
                  ) : (
                    <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: '10px'}}>
                      {dadosDecomposicao.data.map((d: any, idx: number) => (
                        <div 
                           key={idx} 
                           className="card" 
                           style={{width: '100%', height: 'auto', padding: '12px', minHeight: '120px'}}
                           onClick={() => AoClicarNoCaractereDecomposto(d.char)}
                        >
                           <div style={{fontSize: '28px', fontFamily: 'var(--fonte-hanzi)'}}>{d.char}</div>
                           {d.res && d.res.length > 0 ? (
                             <>
                               <div style={{fontSize: '12px', color: 'var(--cor-pinyin)', marginTop: '8px'}}>{d.res[0].Pinyin}</div>
                               <div style={{fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '4px', overflow: 'hidden', textOverflow: 'ellipsis', display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical'}}>
                                 {d.res[0].Significados.join(', ')}
                               </div>
                             </>
                           ) : (
                             <div style={{fontSize: '11px', color: 'var(--cor-texto-suave)', marginTop: '4px'}}>Desconhecido</div>
                           )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Pop-up de aviso de compatibilidade */}
      <ModalAvisoCompatibilidade
        avisoCompatibilidade={avisoCompatibilidade}
        setAvisoCompatibilidade={setAvisoCompatibilidade}
      />

      {/* Modal de confirmação */}
      <ModalConfirmacao
        confirmacao={confirmacao}
        setConfirmacao={setConfirmacao}
      />

      {/* Modal Adicionar Hanzi Manualmente */}
      <ModalAdicionarHanzi
        modalAdicionarHanzi={modalAdicionarHanzi}
        setModalAdicionarHanzi={setModalAdicionarHanzi}
        inputAdicionarHanzi={inputAdicionarHanzi}
        setInputAdicionarHanzi={setInputAdicionarHanzi}
        sugestoesPinyin={sugestoesPinyin}
        setSugestoesPinyin={setSugestoesPinyin}
        SalvarPalavra={SalvarPalavra}
        setStatus={setStatus}
      />

    </div>
  );
}

export default App;
