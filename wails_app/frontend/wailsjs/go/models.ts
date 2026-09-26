export namespace config {
	
	export class Config {
	    intervaloCapturaSegundos: number;
	    autoScanAtivo: boolean;
	    confiancaMinimaOcr: number;
	    threadsCpuOcr: number;
	    hardwareSelecionado: string;
	    dispositivoOcr: string;
	    modeloOcr: string;
	    motorOcrAtivo: string;
	    escalaResolucaoOcr: number;
	    limitarPorUsoCpu: boolean;
	    usoMaximoCpuPercent: number;
	    limitarPorUsoGpu: boolean;
	    usoMaximoGpuPercent: number;
	    distanciaMaximaHoverPx: number;
	    intervaloAtualizacaoHoverMs: number;
	    habilitarPopupHover: boolean;
	    tempoParadoPopupMs: number;
	    destacarEstudoTela: boolean;
	    destacarEstudoParcialTela: boolean;
	    monitorAlvo: number;
	    atalhoEscanear: string;
	    atalhoPopupTodos: string;
	    atalhoMarcarEstudo: string;
	    atalhoAlternarPopupHover: string;
	    traducaoApiKey: string;
	    traducaoAtiva: boolean;
	    traducaoPausarPorCota: boolean;
	    traducaoLimiteCotaPercent: number;
	    traducaoUsarCache: boolean;
	    geminiApiKey: string;
	    geminiAtivo: boolean;
	    geminiPopupResumo: boolean;
	    geminiPopupLinha: boolean;
	    geminiCantoResumo: string;
	    geminiEnviarImagem: boolean;
	    geminiPausarPorCota: boolean;
	    geminiLimiteRequisicoesDia: number;
	    geminiModelo: string;
	    censurarJanelasDoApp: boolean;
	    habilitarLeituraPinyin: boolean;
	    lerPinyinAoAbrirPopup: boolean;
	    lerPinyinAoExpandirCard: boolean;
	    lerPinyinAoCompletarDesenho: boolean;
	    motorTtsAtivo: string;
	    motorSttAtivo: string;
	    priorizarEstudoRevisao: boolean;
	    tamanhoFocoRevisao: number;
	    tamanhoFocoAutomatico: boolean;
	    sonsRevisao: boolean;
	    modosRevisaoGeralDesativados: string[];
	    atividadesDesativadas: string[];
	    revisaoFiltroTema: string;
	    revisaoFiltroDificuldade: string;
	    revisaoQuantidadeQuestoes: number;
	    revisarErradasAoFinal: boolean;
	    revisaoIaVibe: string;
	    tipoHanziGerado: string;
	    tipoHanziExibicao: string;
	    restringirHanziDesenho: boolean;
	    mostrarSugestaoPalavrasVistas: boolean;
	    vigiaCardsAtivo: boolean;
	    rastrearPalavrasPerdidas: boolean;
	    idiomaTraducao: string;
	    canalAtualizacao: string;

	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.intervaloCapturaSegundos = source["intervaloCapturaSegundos"];
	        this.autoScanAtivo = source["autoScanAtivo"];
	        this.confiancaMinimaOcr = source["confiancaMinimaOcr"];
	        this.threadsCpuOcr = source["threadsCpuOcr"];
	        this.hardwareSelecionado = source["hardwareSelecionado"];
	        this.dispositivoOcr = source["dispositivoOcr"];
	        this.modeloOcr = source["modeloOcr"];
	        this.motorOcrAtivo = source["motorOcrAtivo"];
	        this.escalaResolucaoOcr = source["escalaResolucaoOcr"];
	        this.limitarPorUsoCpu = source["limitarPorUsoCpu"];
	        this.usoMaximoCpuPercent = source["usoMaximoCpuPercent"];
	        this.limitarPorUsoGpu = source["limitarPorUsoGpu"];
	        this.usoMaximoGpuPercent = source["usoMaximoGpuPercent"];
	        this.distanciaMaximaHoverPx = source["distanciaMaximaHoverPx"];
	        this.intervaloAtualizacaoHoverMs = source["intervaloAtualizacaoHoverMs"];
	        this.habilitarPopupHover = source["habilitarPopupHover"];
	        this.tempoParadoPopupMs = source["tempoParadoPopupMs"];
	        this.destacarEstudoTela = source["destacarEstudoTela"];
	        this.destacarEstudoParcialTela = source["destacarEstudoParcialTela"];
	        this.monitorAlvo = source["monitorAlvo"];
	        this.atalhoEscanear = source["atalhoEscanear"];
	        this.atalhoPopupTodos = source["atalhoPopupTodos"];
	        this.atalhoMarcarEstudo = source["atalhoMarcarEstudo"];
	        this.atalhoAlternarPopupHover = source["atalhoAlternarPopupHover"];
	        this.traducaoApiKey = source["traducaoApiKey"];
	        this.traducaoAtiva = source["traducaoAtiva"];
	        this.traducaoPausarPorCota = source["traducaoPausarPorCota"];
	        this.traducaoLimiteCotaPercent = source["traducaoLimiteCotaPercent"];
	        this.traducaoUsarCache = source["traducaoUsarCache"];
	        this.geminiApiKey = source["geminiApiKey"];
	        this.geminiAtivo = source["geminiAtivo"];
	        this.geminiPopupResumo = source["geminiPopupResumo"];
	        this.geminiPopupLinha = source["geminiPopupLinha"];
	        this.geminiCantoResumo = source["geminiCantoResumo"];
	        this.geminiEnviarImagem = source["geminiEnviarImagem"];
	        this.geminiPausarPorCota = source["geminiPausarPorCota"];
	        this.geminiLimiteRequisicoesDia = source["geminiLimiteRequisicoesDia"];
	        this.geminiModelo = source["geminiModelo"];
	        this.censurarJanelasDoApp = source["censurarJanelasDoApp"];
	        this.habilitarLeituraPinyin = source["habilitarLeituraPinyin"];
	        this.lerPinyinAoAbrirPopup = source["lerPinyinAoAbrirPopup"];
	        this.lerPinyinAoExpandirCard = source["lerPinyinAoExpandirCard"];
	        this.lerPinyinAoCompletarDesenho = source["lerPinyinAoCompletarDesenho"];
	        this.motorTtsAtivo = source["motorTtsAtivo"];
	        this.motorSttAtivo = source["motorSttAtivo"];
	        this.priorizarEstudoRevisao = source["priorizarEstudoRevisao"];
	        this.tamanhoFocoRevisao = source["tamanhoFocoRevisao"];
        this.tamanhoFocoAutomatico = source["tamanhoFocoAutomatico"];
	        this.sonsRevisao = source["sonsRevisao"];
	        this.modosRevisaoGeralDesativados = source["modosRevisaoGeralDesativados"];
	        this.atividadesDesativadas = source["atividadesDesativadas"];
	        this.revisaoFiltroTema = source["revisaoFiltroTema"];
	        this.revisaoFiltroDificuldade = source["revisaoFiltroDificuldade"];
	        this.revisaoQuantidadeQuestoes = source["revisaoQuantidadeQuestoes"];
	        this.revisarErradasAoFinal = source["revisarErradasAoFinal"];
	        this.revisaoIaVibe = source["revisaoIaVibe"];
	        this.tipoHanziGerado = source["tipoHanziGerado"];
	        this.tipoHanziExibicao = source["tipoHanziExibicao"];
	        this.restringirHanziDesenho = source["restringirHanziDesenho"];
	        this.mostrarSugestaoPalavrasVistas = source["mostrarSugestaoPalavrasVistas"];
	        this.vigiaCardsAtivo = source["vigiaCardsAtivo"];
	        this.rastrearPalavrasPerdidas = source["rastrearPalavrasPerdidas"];
	        this.idiomaTraducao = source["idiomaTraducao"];
	        this.canalAtualizacao = source["canalAtualizacao"];
	    }
	}

}

export namespace dicionario {
	
	export class Etimologia {
	    type?: string;
	    phonetic?: string;
	    semantic?: string;
	    hint?: string;
	
	    static createFrom(source: any = {}) {
	        return new Etimologia(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.phonetic = source["phonetic"];
	        this.semantic = source["semantic"];
	        this.hint = source["hint"];
	    }
	}
	export class DecomposicaoHanzi {
	    character: string;
	    definition?: string;
	    pinyin?: string[];
	    decomposition: string;
	    etymology?: Etimologia;
	    radical: string;
	    abreviacoes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new DecomposicaoHanzi(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.character = source["character"];
	        this.definition = source["definition"];
	        this.pinyin = source["pinyin"];
	        this.decomposition = source["decomposition"];
	        this.etymology = this.convertValues(source["etymology"], Etimologia);
	        this.radical = source["radical"];
	        this.abreviacoes = source["abreviacoes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ComponenteHanzi {
	    caractere: string;
	    tracos: number[];

	    static createFrom(source: any = {}) {
	        return new ComponenteHanzi(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.caractere = source["caractere"];
	        this.tracos = source["tracos"];
	    }
	}
	export class EntradaDicionario {
	    Tradicional: string;
	    Simplificado: string;
	    Pinyin: string;
	    Significados: string[];
	
	    static createFrom(source: any = {}) {
	        return new EntradaDicionario(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Tradicional = source["Tradicional"];
	        this.Simplificado = source["Simplificado"];
	        this.Pinyin = source["Pinyin"];
	        this.Significados = source["Significados"];
	    }
	}

}

export namespace main {
	
	export class EstadoAtualizacao {
	    versaoAtual: string;
	    canalBuild: string;
	    commit: string;
	    buildLocal: boolean;
	    fase: string;
	    versaoAlvo: string;
	    erro: string;

	    static createFrom(source: any = {}) {
	        return new EstadoAtualizacao(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.versaoAtual = source["versaoAtual"];
	        this.canalBuild = source["canalBuild"];
	        this.commit = source["commit"];
	        this.buildLocal = source["buildLocal"];
	        this.fase = source["fase"];
	        this.versaoAlvo = source["versaoAlvo"];
	        this.erro = source["erro"];
	    }
	}
	export class ResultadoVerificacaoAtualizacao {
	    disponivel: boolean;
	    versaoAlvo: string;
	    motivo: string;

	    static createFrom(source: any = {}) {
	        return new ResultadoVerificacaoAtualizacao(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.disponivel = source["disponivel"];
	        this.versaoAlvo = source["versaoAlvo"];
	        this.motivo = source["motivo"];
	    }
	}
	export class ArquivoModelo {
	    nome: string;
	    url: string;
	    sha256: string;
	
	    static createFrom(source: any = {}) {
	        return new ArquivoModelo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.url = source["url"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class FlashcardCard {
	    hanzi: string;
	    pinyin: string;
	    significados: string[];
	    confianca: number;
	    caixa: number[];
	    imageId?: number;
	    tipoHanzi: string;
	    posicaoRanking: number;
	    nivelHSK?: number;
	    fantasma?: boolean;

	    static createFrom(source: any = {}) {
	        return new FlashcardCard(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hanzi = source["hanzi"];
	        this.pinyin = source["pinyin"];
	        this.significados = source["significados"];
	        this.confianca = source["confianca"];
	        this.caixa = source["caixa"];
	        this.imageId = source["imageId"];
	        this.tipoHanzi = source["tipoHanzi"];
	        this.posicaoRanking = source["posicaoRanking"];
	        this.nivelHSK = source["nivelHSK"];
	        this.fantasma = source["fantasma"];
	    }
	}
	export class InfoCotaGemini {
	    requisicoesUsadas: number;
	    data: string;
	
	    static createFrom(source: any = {}) {
	        return new InfoCotaGemini(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requisicoesUsadas = source["requisicoesUsadas"];
	        this.data = source["data"];
	    }
	}
	export class InfoCotaTraducao {
	    caracteresUsados: number;
	    cotaTotal: number;
	    percentual: number;
	    anoMes: string;
	
	    static createFrom(source: any = {}) {
	        return new InfoCotaTraducao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.caracteresUsados = source["caracteresUsados"];
	        this.cotaTotal = source["cotaTotal"];
	        this.percentual = source["percentual"];
	        this.anoMes = source["anoMes"];
	    }
	}
	export class InformacaoExpansao {
	    posicaoRanking: number;
	    alternativa: string;
	    tipoHanzi: string;
	    nivelHSK?: number;
	
	    static createFrom(source: any = {}) {
	        return new InformacaoExpansao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.posicaoRanking = source["posicaoRanking"];
	        this.alternativa = source["alternativa"];
	        this.tipoHanzi = source["tipoHanzi"];
	        this.nivelHSK = source["nivelHSK"];
	    }
	}
	export class RecomendacaoBaralho {
	    hanzi: string;
	    pinyin: string;
	    significado: string;
	    motivo: string;
	    revelada: boolean;
	    nivelHSK?: number;
	    posicaoRanking?: number;
	
	    static createFrom(source: any = {}) {
	        return new RecomendacaoBaralho(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hanzi = source["hanzi"];
	        this.pinyin = source["pinyin"];
	        this.significado = source["significado"];
	        this.motivo = source["motivo"];
	        this.revelada = source["revelada"];
	        this.nivelHSK = source["nivelHSK"];
	        this.posicaoRanking = source["posicaoRanking"];
	    }
	}
	export class ItemArmazenamento {
	    chave: string;
	    rotulo: string;
	    descricao: string;
	    caminho: string;
	    bytes: number;
	    limpavel: boolean;
	    perigoso: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ItemArmazenamento(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chave = source["chave"];
	        this.rotulo = source["rotulo"];
	        this.descricao = source["descricao"];
	        this.caminho = source["caminho"];
	        this.bytes = source["bytes"];
	        this.limpavel = source["limpavel"];
	        this.perigoso = source["perigoso"];
	    }
	}
	export class ModeloOcrInfo {
	    nome: string;
	    rotulo: string;
	    descricao: string;
	    idiomas: string[];
	    baixavel: boolean;
	    embutido: boolean;
	    instalado: boolean;
	    tamanhoBytes: number;
	    arquivos: ArquivoModelo[];
	
	    static createFrom(source: any = {}) {
	        return new ModeloOcrInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.rotulo = source["rotulo"];
	        this.descricao = source["descricao"];
	        this.idiomas = source["idiomas"];
	        this.baixavel = source["baixavel"];
	        this.embutido = source["embutido"];
	        this.instalado = source["instalado"];
	        this.tamanhoBytes = source["tamanhoBytes"];
	        this.arquivos = this.convertValues(source["arquivos"], ArquivoModelo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Monitor {
	    id: number;
	    nome: string;
	    largura: number;
	    altura: number;
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new Monitor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.nome = source["nome"];
	        this.largura = source["largura"];
	        this.altura = source["altura"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class MotorOcrInfo {
	    nome: string;
	    rotulo: string;
	    descricao: string;
	    idiomas: string[];
	    versao: string;
	    variante: string;
	    requisitos: string;
	    padrao: boolean;
	    tamanhoBytes: number;
	    instalado: boolean;
	    ativo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MotorOcrInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.rotulo = source["rotulo"];
	        this.descricao = source["descricao"];
	        this.idiomas = source["idiomas"];
	        this.versao = source["versao"];
	        this.variante = source["variante"];
	        this.requisitos = source["requisitos"];
	        this.padrao = source["padrao"];
	        this.tamanhoBytes = source["tamanhoBytes"];
	        this.instalado = source["instalado"];
	        this.ativo = source["ativo"];
	    }
	}
	export class MotorSttInfo {
	    nome: string;
	    rotulo: string;
	    descricao: string;
	    versao: string;
	    requisitos: string;
	    tamanhoBytes: number;
	    publicado: boolean;
	    instalado: boolean;
	    ativo: boolean;

	    static createFrom(source: any = {}) {
	        return new MotorSttInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.rotulo = source["rotulo"];
	        this.descricao = source["descricao"];
	        this.versao = source["versao"];
	        this.requisitos = source["requisitos"];
	        this.tamanhoBytes = source["tamanhoBytes"];
	        this.publicado = source["publicado"];
	        this.instalado = source["instalado"];
	        this.ativo = source["ativo"];
	    }
	}
	export class MotorTtsInfo {
	    nome: string;
	    rotulo: string;
	    descricao: string;
	    versao: string;
	    requisitos: string;
	    tamanhoBytes: number;
	    publicado: boolean;
	    instalado: boolean;
	    ativo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MotorTtsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nome = source["nome"];
	        this.rotulo = source["rotulo"];
	        this.descricao = source["descricao"];
	        this.versao = source["versao"];
	        this.requisitos = source["requisitos"];
	        this.tamanhoBytes = source["tamanhoBytes"];
	        this.publicado = source["publicado"];
	        this.instalado = source["instalado"];
	        this.ativo = source["ativo"];
	    }
	}
	export class ItemFocoRevisao {
	    hanzi: string;
	    pinyin: string;
	    significados: string[];
	    areasConcluidas: number;
 
	    static createFrom(source: any = {}) {
	        return new ItemFocoRevisao(source);
	    }
 
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hanzi = source["hanzi"];
	        this.pinyin = source["pinyin"];
	        this.significados = source["significados"];
	        this.areasConcluidas = source["areasConcluidas"];
	    }
	}
	export class ProgressoPalavraRevisao {
	    hanzi: string;
	    pinyin: string;
	    significados: string[];
	    status: string;
	    streaks: {[key: string]: number};

	    static createFrom(source: any = {}) {
	        return new ProgressoPalavraRevisao(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hanzi = source["hanzi"];
	        this.pinyin = source["pinyin"];
	        this.significados = source["significados"];
	        this.status = source["status"];
	        this.streaks = source["streaks"];
	    }
	}
	export class ProgressoRevisaoPalavras {
	    meta: number;
	    palavras: ProgressoPalavraRevisao[];

	    static createFrom(source: any = {}) {
	        return new ProgressoRevisaoPalavras(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.meta = source["meta"];
	        this.palavras = source["palavras"];
	    }
	}
	export class ElementoOrdenacao {
	    texto: string;
	    pinyin: string;
	    definicao: string;
	    correta: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ElementoOrdenacao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.texto = source["texto"];
	        this.pinyin = source["pinyin"];
	        this.definicao = source["definicao"];
	        this.correta = source["correta"];
	    }
	}
	export class OpcaoRevisao {
	    hanzi: string;
	    pinyin: string;
	    definicao: string;
	    correta: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OpcaoRevisao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hanzi = source["hanzi"];
	        this.pinyin = source["pinyin"];
	        this.definicao = source["definicao"];
	        this.correta = source["correta"];
	    }
	}
	export class PalavraRevisao {
	    texto: string;
	    pinyin: string;
	    significados: string[];
	    ehChines: boolean;
	    ehLacuna?: boolean;
	    ehNaoVista?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PalavraRevisao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.texto = source["texto"];
	        this.pinyin = source["pinyin"];
	        this.significados = source["significados"];
	        this.ehChines = source["ehChines"];
	        this.ehLacuna = source["ehLacuna"];
	        this.ehNaoVista = source["ehNaoVista"];
	    }
	}
	export class QuestaoRevisao {
	    modo: string;
	    variante: string;
	    hanzi: string;
	    palavraFoco: string;
	    pinyin: string;
	    definicao: string;
	    componenteAlvo: string;
	    tracosAlvo: number[];
	    componentesMontagem: dicionario.ComponenteHanzi[];
	    distratoresMontagem: string[];
	    emEstudo: boolean;
	    emFoco: boolean;
	    fraseLacuna: string;
	    fraseOriginal: string;
	    fraseOculta: string;
	    fraseLacunaSegmentada: PalavraRevisao[];
	    fraseOriginalSegmentada: PalavraRevisao[];
	    fraseTraducao: string;
	    fraseAtribuicao: string;
	    fraseTema: string;
	    fraseDificuldade: string;
	    dificuldade: string;
	    opcoes: OpcaoRevisao[];
	    pilhaOrdenacao: OpcaoRevisao[];
	    pecasEsperadas: string[];
	    elementosOrdenacao: ElementoOrdenacao[];
	    perguntaCompreensao?: string;
	    perguntaCompreensaoSegmentada?: PalavraRevisao[];
	    perguntaTraduzida?: string;
	    contextoTraduzido?: string;
	    indiceRespostaCorreta?: number;
	
	    static createFrom(source: any = {}) {
	        return new QuestaoRevisao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modo = source["modo"];
	        this.variante = source["variante"];
	        this.hanzi = source["hanzi"];
	        this.palavraFoco = source["palavraFoco"];
	        this.pinyin = source["pinyin"];
	        this.definicao = source["definicao"];
	        this.componenteAlvo = source["componenteAlvo"];
	        this.tracosAlvo = source["tracosAlvo"];
	        this.componentesMontagem = this.convertValues(source["componentesMontagem"], dicionario.ComponenteHanzi);
	        this.distratoresMontagem = source["distratoresMontagem"];
	        this.emEstudo = source["emEstudo"];
	        this.emFoco = source["emFoco"];
	        this.fraseLacuna = source["fraseLacuna"];
	        this.fraseOriginal = source["fraseOriginal"];
	        this.fraseOculta = source["fraseOculta"];
	        this.fraseLacunaSegmentada = this.convertValues(source["fraseLacunaSegmentada"], PalavraRevisao);
	        this.fraseOriginalSegmentada = this.convertValues(source["fraseOriginalSegmentada"], PalavraRevisao);
	        this.fraseTraducao = source["fraseTraducao"];
	        this.fraseAtribuicao = source["fraseAtribuicao"];
	        this.fraseTema = source["fraseTema"];
	        this.fraseDificuldade = source["fraseDificuldade"];
	        this.dificuldade = source["dificuldade"];
	        this.opcoes = this.convertValues(source["opcoes"], OpcaoRevisao);
	        this.pilhaOrdenacao = this.convertValues(source["pilhaOrdenacao"], OpcaoRevisao);
	        this.pecasEsperadas = source["pecasEsperadas"];
	        this.elementosOrdenacao = this.convertValues(source["elementosOrdenacao"], ElementoOrdenacao);
	        this.perguntaCompreensao = source["perguntaCompreensao"];
	        this.perguntaCompreensaoSegmentada = this.convertValues(source["perguntaCompreensaoSegmentada"], PalavraRevisao);
	        this.perguntaTraduzida = source["perguntaTraduzida"];
	        this.contextoTraduzido = source["contextoTraduzido"];
	        this.indiceRespostaCorreta = source["indiceRespostaCorreta"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Resolucao {
	    largura: number;
	    altura: number;
	
	    static createFrom(source: any = {}) {
	        return new Resolucao(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.largura = source["largura"];
	        this.altura = source["altura"];
	    }
	}
	export class StorageInfo {
	    itens: ItemArmazenamento[];
	    totalBytes: number;
	    discoLivre: number;
	    discoTotal: number;
	    pastaDados: string;
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itens = this.convertValues(source["itens"], ItemArmazenamento);
	        this.totalBytes = source["totalBytes"];
	        this.discoLivre = source["discoLivre"];
	        this.discoTotal = source["discoTotal"];
	        this.pastaDados = source["pastaDados"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SystemHardware {
	    cpu: string;
	    gpus: string[];
	
	    static createFrom(source: any = {}) {
	        return new SystemHardware(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu = source["cpu"];
	        this.gpus = source["gpus"];
	    }
	}

}

export namespace nuvem {
	
	export class Info {
	    estado: string;
	    email: string;
	    ultimaSincronizacao: string;
	    remotoBytes: number;
	    remotoModificadoEm: string;
	    localBytes: number;
	    erro: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.estado = source["estado"];
	        this.email = source["email"];
	        this.ultimaSincronizacao = source["ultimaSincronizacao"];
	        this.remotoBytes = source["remotoBytes"];
	        this.remotoModificadoEm = source["remotoModificadoEm"];
	        this.localBytes = source["localBytes"];
	        this.erro = source["erro"];
	    }
	}

}

export namespace progresso {
	
	export class Vocab {
	    Id: number;
	    Hanzi: string;
	    Pinyin: string;
	    Significado: string;
	    Status: string;
	    // Go type: time
	    DataAdd: any;
	    tipoHanzi: string;
	    vezesVistaOcr: number;
	    posicaoRanking: number;

	    static createFrom(source: any = {}) {
	        return new Vocab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Id = source["Id"];
	        this.Hanzi = source["Hanzi"];
	        this.Pinyin = source["Pinyin"];
	        this.Significado = source["Significado"];
	        this.Status = source["Status"];
	        this.DataAdd = this.convertValues(source["DataAdd"], null);
	        this.tipoHanzi = source["tipoHanzi"];
	        this.vezesVistaOcr = source["vezesVistaOcr"];
	        this.posicaoRanking = source["posicaoRanking"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class FraseUsuario {
	    Chines: string;
	    Ingles: string;
	    Atribuicao: string;
	    Tema: string;
	    Dificuldade: string;

	    static createFrom(source: any = {}) {
	        return new FraseUsuario(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Chines = source["Chines"];
	        this.Ingles = source["Ingles"];
	        this.Atribuicao = source["Atribuicao"];
	        this.Tema = source["Tema"];
	        this.Dificuldade = source["Dificuldade"];
	    }
	}

}

export namespace notasversao {
	
	export class GrupoNota {
	    titulo: string;
	    itens: string[];

	    static createFrom(source: any = {}) {
	        return new GrupoNota(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.titulo = source["titulo"];
	        this.itens = source["itens"];
	    }
	}

	export class Nota {
	    id: string;
	    data: string;
	    titulo: string;
	    grupos: GrupoNota[];

	    static createFrom(source: any = {}) {
	        return new Nota(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.data = source["data"];
	        this.titulo = source["titulo"];
	        this.grupos = this.convertValues(source["grupos"], GrupoNota);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}
