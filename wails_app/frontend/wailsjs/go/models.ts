export namespace config {
	
	export class Config {
	    intervaloCapturaSegundos: number;
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
	    censurarJanelasDoApp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.intervaloCapturaSegundos = source["intervaloCapturaSegundos"];
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
	        this.censurarJanelasDoApp = source["censurarJanelasDoApp"];
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

export namespace progresso {
	
	export class Vocab {
	    Id: number;
	    Hanzi: string;
	    Pinyin: string;
	    Significado: string;
	    Status: string;
	    // Go type: time
	    DataAdd: any;
	
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

