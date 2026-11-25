// Package lingvoj lists the meaning languages a learner can pick (the
// language vocab cards show their meaning in) with their own names.
//
// The list is the course's own definition languages plus every language in
// which Reta Vortaro translates at least ~40% of the course's vocab cards
// (internal/revo TestCourseCoverage measures it). Languages ReVo covers
// less than that would mostly show the English fallback.
package lingvoj

import "sort"

// Names are the languages' names in themselves.
var Names = map[string]string{
	"af": "Afrikaans", "am": "አማርኛ", "ar": "العربية", "az": "Azərbaycanca",
	"be": "Беларуская", "bg": "Български", "bn": "বাংলা", "bo": "བོད་ཡིག",
	"bs": "Bosanski", "ca": "Català", "co": "Corsu", "cs": "Čeština",
	"cy": "Cymraeg", "da": "Dansk", "de": "Deutsch", "el": "Ελληνικά",
	"en": "English", "eo": "Esperanto", "es": "Español", "et": "Eesti",
	"eu": "Euskara", "fa": "فارسی", "fr": "Français", "frp": "Arpitan",
	"fy": "Frysk", "ga": "Gaeilge", "gd": "Gàidhlig", "gl": "Galego",
	"gu": "ગુજરાતી", "ha": "Hausa", "he": "עברית", "hi": "हिन्दी",
	"hmn": "Hmoob", "hr": "Hrvatski", "ht": "Kreyòl ayisyen", "hu": "Magyar",
	"hy": "Հայերեն", "id": "Bahasa Indonesia", "ig": "Igbo", "is": "Íslenska",
	"it": "Italiano", "ja": "日本語", "jv": "Basa Jawa", "ka": "ქართული",
	"kk": "Қазақша", "km": "ខ្មែរ", "kn": "ಕನ್ನಡ", "ko": "한국어",
	"ku": "Kurdî", "ky": "Кыргызча", "lo": "ລາວ", "lt": "Lietuvių",
	"lv": "Latviešu", "mg": "Malagasy", "mi": "Te Reo Māori", "mk": "Македонски",
	"ml": "മലയാളം", "mn": "Монгол", "mr": "मराठी", "ms": "Bahasa Melayu",
	"mt": "Malti", "my": "မြန်မာဘာသာ", "ne": "नेपाली", "nl": "Nederlands",
	"ny": "Chichewa", "pa": "ਪੰਜਾਬੀ", "pl": "Polski", "ps": "پښتو",
	"pt": "Português", "ro": "Română", "ru": "Русский", "rw": "Kinyarwanda",
	"sd": "سنڌي", "si": "සිංහල", "sk": "Slovenčina", "sl": "Slovenščina",
	"sm": "Gagana Sāmoa", "sn": "chiShona", "so": "Soomaali", "sq": "Shqip",
	"st": "Sesotho", "su": "Basa Sunda", "sv": "Svenska", "sw": "Kiswahili",
	"ta": "தமிழ்", "te": "తెలుగు", "tg": "Тоҷикӣ", "th": "ไทย",
	"tl": "Tagalog", "tok": "Toki Pona", "tr": "Türkçe", "tt": "Татарча",
	"uk": "Українська", "ur": "اردو", "uz": "Oʻzbekcha", "vi": "Tiếng Việt",
	"xh": "isiXhosa", "yi": "ייִדיש", "yo": "Yorùbá", "zh": "中文",
	"zh-tw": "繁體中文", "zu": "isiZulu",
}

// Name is a language's own name, or the code when it has none here.
func Name(code string) string {
	if n, ok := Names[code]; ok {
		return n
	}
	return code
}

// Lang is one entry of the meaning-language picker.
type Lang struct{ Code, Name string }

var vocab []Lang

func init() {
	for c, n := range Names {
		vocab = append(vocab, Lang{c, n})
	}
	sort.Slice(vocab, func(i, j int) bool { return vocab[i].Code < vocab[j].Code })
}

// Vocab is every meaning language, sorted by code.
func Vocab() []Lang { return vocab }
