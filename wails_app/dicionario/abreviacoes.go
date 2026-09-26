package dicionario

// ----- Abreviações visuais de radical → caractere CJK completo -----
//
// Caracteres do bloco CJK Radicals Supplement e componentes que só aparecem colados dentro de outros
// hanzi. Não são palavras: não se pronunciam sozinhos, não viram card e não entram no sorteio da
// revisão nem no cache de TTS — o mapa é o que permite reconhecê-los e resolver para o caractere
// pronunciável equivalente.
var MapaAbrevParaCompleto = map[string]string{
	// CJK Radicals Supplement
	"⺀": "冫", // gelo
	"⺈": "刀", // faca
	"⺊": "卜", // adivinhação
	"⺌": "小", // pequeno (forma superior)
	"⺍": "小", // pequeno (variante)
	"⺗": "心", // coração (forma inferior)
	"⺮": "竹", // bambu (forma superior)
	"⺳": "网", // rede (forma superior)
	"⺼": "肉", // carne (parece 月)

	// Componentes e radicais avulsos (para não virarem cards separados)
	"氵": "水",
	"冫": "冰",
	"亻": "人",
	"艹": "草",
	"扌": "手",
	"阝": "邑", // ou 阜, mapeado para 邑 convencionalmente
	"犭": "犬",
	"忄": "心",
	"辶": "走",
	"廴": "建",
	"彳": "行",
	"刂": "刀",
	"灬": "火",
	"糹": "糸",
	"纟": "糸",
	"釒": "金",
	"钅": "金",
	"飠": "食",
	"饣": "食",
	"衤": "衣",
	"礻": "示",
	"疒": "病",
	"罒": "网",
	"讠": "言",
}
