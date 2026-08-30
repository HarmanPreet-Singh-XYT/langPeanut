package llm

import (
	"fmt"
	"regexp"
	"strings"
)

// bcp47ToFlores200 maps ISO-639 / BCP-47 language codes and names to Meta NLLB-200 (FLORES-200) language codes
var bcp47ToFlores200 = map[string]string{
	// Common World Languages
	"en":       "eng_Latn",
	"en-US":    "eng_Latn",
	"en-GB":    "eng_Latn",
	"es":       "spa_Latn",
	"es-ES":    "spa_Latn",
	"es-MX":    "spa_Latn",
	"es-419":   "spa_Latn",
	"fr":       "fra_Latn",
	"fr-FR":    "fra_Latn",
	"fr-CA":    "fra_Latn",
	"de":       "deu_Latn",
	"de-DE":    "deu_Latn",
	"de-AT":    "deu_Latn",
	"de-CH":    "deu_Latn",
	"it":       "ita_Latn",
	"pt":       "por_Latn",
	"pt-BR":    "por_Latn",
	"pt-PT":    "por_Latn",
	"ja":       "jpn_Jpan",
	"zh":       "zho_Hans",
	"zh-CN":    "zho_Hans",
	"zh-Hans":  "zho_Hans",
	"zh-TW":    "zho_Hant",
	"zh-HK":    "zho_Hant",
	"zh-Hant":  "zho_Hant",
	"hi":       "hin_Deva",
	"pa":       "pan_Guru",
	"ar":       "ara_Arab",
	"ru":       "rus_Cyrl",
	"ko":       "kor_Hang",
	"nl":       "nld_Latn",
	"tr":       "tur_Latn",
	"pl":       "pol_Latn",
	"sv":       "swe_Latn",
	"vi":       "vie_Latn",
	"th":       "tha_Thai",
	"id":       "ind_Latn",
	"uk":       "ukr_Cyrl",
	"he":       "heb_Hebr",
	"el":       "ell_Grek",
	"cs":       "ces_Latn",
	"da":       "dan_Latn",
	"fi":       "fin_Latn",
	"hu":       "hun_Latn",
	"no":       "nob_Latn",
	"nb":       "nob_Latn",
	"nn":       "nno_Latn",
	"ro":       "ron_Latn",
	"sk":       "slk_Latn",
	"bg":       "bul_Cyrl",
	"hr":       "hrv_Latn",
	"sr":       "srp_Cyrl",
	"sl":       "slv_Latn",
	"lt":       "lit_Latn",
	"lv":       "lvs_Latn",
	"et":       "est_Latn",
	"ms":       "zsm_Latn",
	"tl":       "tgl_Latn",
	"fil":      "tgl_Latn",
	"sw":       "swh_Latn",
	"bn":       "ben_Beng",
	"ta":       "tam_Taml",
	"te":       "tel_Telu",
	"mr":       "mar_Deva",
	"gu":       "guj_Gujr",
	"kn":       "kan_Knda",
	"ml":       "mal_Mlym",
	"ur":       "urd_Arab",
	"fa":       "pes_Arab",
	"af":       "afr_Latn",
	"sq":       "sqi_Latn",
	"hy":       "hye_Armn",
	"az":       "azj_Latn",
	"eu":       "eus_Latn",
	"be":       "bel_Cyrl",
	"bs":       "bos_Latn",
	"ca":       "cat_Latn",
	"ka":       "kat_Geor",
	"gl":       "glg_Latn",
	"is":       "isl_Latn",
	"ga":       "gle_Latn",
	"kk":       "kaz_Cyrl",
	"km":       "khm_Khmr",
	"lo":       "lao_Laoo",
	"mk":       "mkd_Cyrl",
	"mn":       "khk_Cyrl",
	"mya":      "mya_Mymr",
	"ne":       "npi_Deva",
	"si":       "sin_Sinh",
	"so":       "som_Latn",
	"uz":       "uzn_Latn",
	"cy":       "cym_Latn",
	"am":       "amh_Ethi",
	"jv":       "jav_Latn",
	"su":       "sun_Latn",
	"mg":       "plt_Latn",
	"eo":       "epo_Latn",
	"la":       "lat_Latn",
	"sa":       "san_Deva",
	"sd":       "snd_Arab",
	"ps":       "pus_Arab",
	"ku":       "kmr_Latn",
	"ky":       "kir_Cyrl",
	"tg":       "tgk_Cyrl",
	"tk":       "tuk_Latn",
	"tt":       "tat_Cyrl",
	"ug":       "uig_Arab",
	"ha":       "hau_Latn",
	"yo":       "yor_Latn",
	"ig":       "ibo_Latn",
	"zu":       "zul_Latn",
	"xh":       "xho_Latn",
	"qu":       "quy_Latn",
	"gn":       "grn_Latn",
	"ay":       "ayr_Latn",
}

// flores200ToBcp47 maps FLORES-200 code back to the canonical standard BCP-47 code
var flores200ToBcp47 = map[string]string{
	"eng_Latn": "en", "fra_Latn": "fr", "spa_Latn": "es", "deu_Latn": "de",
	"ita_Latn": "it", "por_Latn": "pt", "jpn_Jpan": "ja", "zho_Hans": "zh",
	"zho_Hant": "zh-TW", "hin_Deva": "hi", "pan_Guru": "pa", "ara_Arab": "ar",
	"rus_Cyrl": "ru", "kor_Hang": "ko", "nld_Latn": "nl", "tur_Latn": "tr",
	"pol_Latn": "pl", "swe_Latn": "sv", "vie_Latn": "vi", "tha_Thai": "th",
	"ind_Latn": "id", "ukr_Cyrl": "uk", "heb_Hebr": "he", "ell_Grek": "el",
	"ces_Latn": "cs", "dan_Latn": "da", "fin_Latn": "fi", "hun_Latn": "hu",
	"nob_Latn": "no", "ron_Latn": "ro", "slk_Latn": "sk", "bul_Cyrl": "bg",
	"ben_Beng": "bn", "tam_Taml": "ta", "tel_Telu": "te", "mar_Deva": "mr",
	"guj_Gujr": "gu", "kan_Knda": "kn", "mal_Mlym": "ml", "urd_Arab": "ur",
}

// ToFlores200 converts any standard locale code (e.g. "fr", "es-ES", "hi", "zh-CN") to NLLB FLORES-200 format (e.g. "fra_Latn")
func ToFlores200(locale string) string {
	clean := strings.TrimSpace(locale)
	if strings.Contains(clean, "_") && len(clean) >= 8 {
		// Already in FLORES-200 format like "fra_Latn"
		return clean
	}

	if code, ok := bcp47ToFlores200[clean]; ok {
		return code
	}

	// Try base code (e.g. "en-US" -> "en")
	if strings.Contains(clean, "-") {
		base := strings.Split(clean, "-")[0]
		if code, ok := bcp47ToFlores200[base]; ok {
			return code
		}
	}
	if strings.Contains(clean, "_") {
		base := strings.Split(clean, "_")[0]
		if code, ok := bcp47ToFlores200[base]; ok {
			return code
		}
	}

	// Case insensitive fallback
	lower := strings.ToLower(clean)
	for k, v := range bcp47ToFlores200 {
		if strings.ToLower(k) == lower {
			return v
		}
	}

	return "eng_Latn"
}

// FromFlores200 converts an NLLB FLORES-200 code back to standard BCP-47
func FromFlores200(floresCode string) string {
	if bcp, ok := flores200ToBcp47[floresCode]; ok {
		return bcp
	}
	// Extract prefix (e.g. "fra_Latn" -> "fra")
	parts := strings.Split(floresCode, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return "en"
}

// placeholderRegex matches ICU variables, React/Next placeholders, Swift format specifiers, and Dart/Kotlin variables
var placeholderRegex = regexp.MustCompile(`(\{[^}]+\}|%[@\w\d\.\$]+|\$[a-zA-Z0-9_]+|\$\{[^}]+\}|\\\(.+?\))`)

// MaskPlaceholders replaces all dynamic tokens in sourceText with non-translatable XML sentinels (e.g. <ph_0/>)
// to guarantee zero variable drift or translation distortion during Seq2Seq NLLB translation.
func MaskPlaceholders(sourceText string) (string, []string) {
	var placeholders []string
	masked := placeholderRegex.ReplaceAllStringFunc(sourceText, func(match string) string {
		idx := len(placeholders)
		placeholders = append(placeholders, match)
		return fmt.Sprintf("<ph_%d/>", idx)
	})
	return masked, placeholders
}

// UnmaskPlaceholders restores original placeholders back into the translated string at the corresponding sentinel locations.
func UnmaskPlaceholders(translatedText string, placeholders []string) string {
	result := translatedText
	for i, ph := range placeholders {
		// Exact tag match
		tag := fmt.Sprintf("<ph_%d/>", i)
		result = strings.ReplaceAll(result, tag, ph)

		// Resilient match in case translation model stripped slash or altered spacing (<ph_0 />, <ph_0>, < ph_0 />)
		altTags := []string{
			fmt.Sprintf("<ph_%d />", i),
			fmt.Sprintf("<ph_%d>", i),
			fmt.Sprintf("< ph_%d />", i),
			fmt.Sprintf("< ph_%d >", i),
			fmt.Sprintf("&lt;ph_%d/&gt;", i),
			fmt.Sprintf("&lt;ph_%d /&gt;", i),
			fmt.Sprintf("&lt;ph_%d&gt;", i),
			fmt.Sprintf("<PH_%d/>", i),
			fmt.Sprintf("<PH_%d>", i),
		}
		for _, alt := range altTags {
			result = strings.ReplaceAll(result, alt, ph)
		}
	}

	// Safety check: If any original placeholder was dropped, append it to prevent runtime syntax breakages
	for _, ph := range placeholders {
		if !strings.Contains(result, ph) {
			result = result + " " + ph
		}
	}

	return result
}

// MaxSafeNLLBTokenCount is the safe token threshold (below NLLB's 512 hard positional embedding limit)
const MaxSafeNLLBTokenCount = 350

// SplitIntoSafeChunks splits long text by paragraph and sentence boundaries if it exceeds MaxSafeNLLBTokenCount words
func SplitIntoSafeChunks(text string, maxWords int) []string {
	if maxWords <= 0 {
		maxWords = MaxSafeNLLBTokenCount
	}
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return []string{text}
	}

	// Split by sentences (period, exclamation, question mark, or newlines)
	sentenceSplitter := regexp.MustCompile(`([^.!?\n]+[.!?\n]+|\S+)`)
	rawSegments := sentenceSplitter.FindAllString(text, -1)
	if len(rawSegments) == 0 {
		return []string{text}
	}

	var chunks []string
	var currentChunk strings.Builder
	currentWordCount := 0

	for _, seg := range rawSegments {
		segWords := len(strings.Fields(seg))
		if currentWordCount+segWords > maxWords && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentWordCount = 0
		}
		currentChunk.WriteString(seg)
		currentWordCount += segWords
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

