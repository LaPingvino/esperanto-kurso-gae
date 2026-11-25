// Package lingvoj lists the meaning languages a learner can pick (the
// language vocab cards show their meaning in) with their own names.
//
// The list is the course's own definition languages plus every language in
// which Reta Vortaro translates at least ~40% of the course's vocab cards
// (internal/revo TestCourseCoverage measures it). Languages ReVo covers
// less than that would mostly show the English fallback.
package lingvoj

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

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

// CopiedFrom lists the languages whose generated course definitions are
// copies of another language's, not translations, with that language:
// measured on the seed, each has the same text as its source on 680 to
// 1,011 of its ~1,011 cards (ur: 199 English + 64 Persian). Elsewhere a
// definition equal to the English one is a cognate (de Hand, nl water), so
// the rule applies only to these.
var CopiedFrom = map[string][]string{
	"ro": {"fr"}, "sw": {"fr"}, "frp": {"fr"}, "be": {"ru"}, "ku": {"fa"},
	"hi": {"en"}, "lo": {"en"}, "my": {"en"}, "yo": {"en"}, "km": {"en"},
	"tok": {"en"}, "kk": {"en"}, "ur": {"en", "fa"},
}

// CopySource returns the language a course definition in lang was copied
// from (defs: the card's definitions per language), or "" when it is a
// definition in its own right.
func CopySource(defs map[string]string, lang string) string {
	d := strings.TrimSpace(defs[lang])
	if d == "" {
		return ""
	}
	for _, src := range CopiedFrom[lang] {
		if strings.EqualFold(d, strings.TrimSpace(defs[src])) {
			return src
		}
	}
	return ""
}

// scripts are the writing systems of the meaning languages that are not
// written in Latin letters (Latin is the default).
var scripts = map[string]*unicode.RangeTable{
	"el": unicode.Greek,
	"be": unicode.Cyrillic, "bg": unicode.Cyrillic, "kk": unicode.Cyrillic, "ky": unicode.Cyrillic,
	"mk": unicode.Cyrillic, "mn": unicode.Cyrillic, "ru": unicode.Cyrillic, "tg": unicode.Cyrillic,
	"tt": unicode.Cyrillic, "uk": unicode.Cyrillic,
	"ar": unicode.Arabic, "fa": unicode.Arabic, "ps": unicode.Arabic, "sd": unicode.Arabic, "ur": unicode.Arabic,
	"he": unicode.Hebrew, "yi": unicode.Hebrew,
	"hi": unicode.Devanagari, "mr": unicode.Devanagari, "ne": unicode.Devanagari,
	"bn": unicode.Bengali, "pa": unicode.Gurmukhi, "gu": unicode.Gujarati, "ta": unicode.Tamil,
	"te": unicode.Telugu, "kn": unicode.Kannada, "ml": unicode.Malayalam, "si": unicode.Sinhala,
	"th": unicode.Thai, "lo": unicode.Lao, "km": unicode.Khmer, "my": unicode.Myanmar, "bo": unicode.Tibetan,
	"ka": unicode.Georgian, "hy": unicode.Armenian, "am": unicode.Ethiopic,
	"zh": unicode.Han, "zh-tw": unicode.Han, "ko": unicode.Hangul,
}

// ForeignLetters counts the letters in s that are not of lang's script
// (Japanese: kana and Han), so a meaning shown as lang's can be checked:
// Greek letters in a German meaning are a wrong-language meaning.
func ForeignLetters(s, lang string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		switch lang {
		case "ja":
			if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) {
				continue
			}
		case "ko":
			if unicode.In(r, unicode.Hangul, unicode.Han) {
				continue
			}
		case "tt": // Tatar is written in Cyrillic and in Latin (Zamanälif)
			if unicode.In(r, unicode.Cyrillic, unicode.Latin) {
				continue
			}
		default:
			t := scripts[lang]
			if t == nil {
				t = unicode.Latin
			}
			if unicode.Is(t, r) {
				continue
			}
		}
		n++
	}
	return n
}

var parenthesized = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)

// WrongScript reports whether s, shown as lang, is mostly in another
// script: more than half its letters, not counting clarifications in
// brackets (ReVo writes those in Esperanto: "каштаваць (manĝaĵojn)").
// A single foreign letter ("літара b", the letter b) is not a wrong
// language; "πολύ" on a German card is.
func WrongScript(s, lang string) bool {
	s = parenthesized.ReplaceAllString(s, "")
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters > 0 && ForeignLetters(s, lang)*2 > letters
}
