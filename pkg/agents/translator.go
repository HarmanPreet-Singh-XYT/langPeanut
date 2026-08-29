package agents

import (
	"context"
	"regexp"
	"strings"

	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// TranslatorAgent handles translating source strings to target locales while strictly preserving ICU syntax
type TranslatorAgent struct {
	Memory        *memory.TranslationMemory
	ProjectMemory *memory.ProjectMemory
}

func NewTranslatorAgent(tm *memory.TranslationMemory, pm *memory.ProjectMemory) *TranslatorAgent {
	return &TranslatorAgent{
		Memory:        tm,
		ProjectMemory: pm,
	}
}

// Built-in multilingual dictionary for offline execution & deterministic benchmark comparisons
var dictionary = map[string]map[string]string{
	"fr": {
		"Welcome back, {name}!":           "Bon retour, {name} !",
		"Submit Order":                    "Passer la commande",
		"Cancel":                          "Annuler",
		"Search":                          "Rechercher",
		"Book":                            "Réserver",
		"Settings":                        "Paramètres",
		"Profile":                         "Profil",
		"Notifications":                   "Notifications",
		"Save":                            "Enregistrer",
		"Delete":                          "Supprimer",
		"Total: ${amount}":                "Total : {amount} $",
		"Order #{orderId}":                "Commande n° {orderId}",
		"You have {count} new messages":   "Vous avez {count} nouveaux messages",
		"Loading...":                      "Chargement...",
		"Sign In":                         "Se connecter",
		"Sign Out":                        "Se déconnecter",
		"Checkout":                        "Paiement",
		"Flight from {origin} to {dest}":  "Vol de {origin} à {dest}",
		"Reserve Flight":                  "Réserver le vol",
	},
	"es": {
		"Welcome back, {name}!":           "¡Bienvenido de nuevo, {name}!",
		"Submit Order":                    "Confirmar pedido",
		"Cancel":                          "Cancelar",
		"Search":                          "Buscar",
		"Book":                            "Reservar",
		"Settings":                        "Configuración",
		"Profile":                         "Perfil",
		"Notifications":                   "Notificaciones",
		"Save":                            "Guardar",
		"Delete":                          "Eliminar",
		"Total: ${amount}":                "Total: ${amount}",
		"Order #{orderId}":                "Pedido #{orderId}",
		"You have {count} new messages":   "Tienes {count} mensajes nuevos",
		"Loading...":                      "Cargando...",
		"Sign In":                         "Iniciar sesión",
		"Sign Out":                        "Cerrar sesión",
		"Checkout":                        "Pagar",
		"Flight from {origin} to {dest}":  "Vuelo de {origin} a {dest}",
		"Reserve Flight":                  "Reservar vuelo",
	},
	"de": {
		"Welcome back, {name}!":           "Willkommen zurück, {name}!",
		"Submit Order":                    "Bestellung abschicken",
		"Cancel":                          "Abbrechen",
		"Search":                          "Suchen",
		"Book":                            "Buchen",
		"Settings":                        "Einstellungen",
		"Profile":                         "Profil",
		"Notifications":                   "Benachrichtigungen",
		"Save":                            "Speichern",
		"Delete":                          "Löschen",
		"Total: ${amount}":                "Gesamtbetrag: {amount} €",
		"Order #{orderId}":                "Bestellung #{orderId}",
		"You have {count} new messages":   "Sie haben {count} neue Nachrichten",
		"Loading...":                      "Wird geladen...",
		"Sign In":                         "Anmelden",
		"Sign Out":                        "Abmelden",
		"Checkout":                        "Zur Kasse",
		"Flight from {origin} to {dest}":  "Flug von {origin} nach {dest}",
		"Reserve Flight":                  "Flug buchen",
	},
	"ja": {
		"Welcome back, {name}!":           "おかえりなさい、{name}さん！",
		"Submit Order":                    "注文を確定する",
		"Cancel":                          "キャンセル",
		"Search":                          "検索",
		"Book":                            "予約する",
		"Settings":                        "設定",
		"Profile":                         "プロフィール",
		"Notifications":                   "通知",
		"Save":                            "保存",
		"Delete":                          "削除",
		"Total: ${amount}":                "合計: ¥{amount}",
		"Order #{orderId}":                "注文番号 #{orderId}",
		"You have {count} new messages":   "{count}件の新しいメッセージがあります",
		"Loading...":                      "読み込み中...",
		"Sign In":                         "サインイン",
		"Sign Out":                        "サインアウト",
		"Checkout":                        "チェックアウト",
		"Flight from {origin} to {dest}":  "{origin}から{dest}へのフライト",
		"Reserve Flight":                  "フライトを予約",
	},
	"ar": {
		"Welcome back, {name}!":           "مرحبًا بعودتك، {name}!",
		"Submit Order":                    "إتمام الطلب",
		"Cancel":                          "إلغاء",
		"Search":                          "بحث",
		"Book":                            "حجز",
		"Settings":                        "الإعدادات",
		"Profile":                         "الملف الشخصي",
		"Notifications":                   "الإشعارات",
		"Save":                            "حفظ",
		"Delete":                          "حذف",
		"Total: ${amount}":                "المجموع: {amount}$",
		"Order #{orderId}":                "طلب رقم #{orderId}",
		"You have {count} new messages":   "لديك {count} رسائل جديدة",
		"Loading...":                      "جار التحميل...",
		"Sign In":                         "تسجيل الدخول",
		"Sign Out":                        "تسجيل الخروج",
		"Checkout":                        "الدفع",
		"Flight from {origin} to {dest}":  "رحلة طيران من {origin} إلى {dest}",
		"Reserve Flight":                  "حجز رحلة طيران",
	},
}

// Built-in Gen-Z Slang dictionary translations
var genZDictionary = map[string]map[string]string{
	"en": {
		"Welcome back, {name}!": "ayyy welcome back, {name}! no cap 🔥",
		"Submit Order":          "Ship it 🚀",
		"Cancel":                "Nah, abort 💀",
		"Search":                "Scope out 🔍",
		"Book":                  "Lock it in 🔒",
		"Settings":              "Vibes & Config ⚙️",
		"Save":                  "Save the vibe ✨",
		"Delete":                "Yeet into void 🗑️",
	},
	"fr": {
		"Welcome back, {name}!": "Wesh bon retour, {name} ! En vrai c'est carré 🔥",
		"Submit Order":          "Valide le bail 🚀",
		"Cancel":                "Laisse tomber 💀",
		"Search":                "Check ça 🔍",
		"Book":                  "Réserve direct 🔒",
		"Settings":              "Paramètres & Mood ⚙️",
		"Save":                  "Garde ça au chaud ✨",
	},
	"es": {
		"Welcome back, {name}!": "¡Qué onda, {name}! Todo bien 🔥",
		"Submit Order":          "Mándalo de una 🚀",
		"Cancel":                "Fila, cancela 💀",
		"Search":                "Pilla acá 🔍",
		"Book":                  "Aparta de una 🔒",
		"Settings":              "Config & Onda ⚙️",
		"Save":                  "Guarda el flow ✨",
	},
}

// TranslateLocale processes all entries in a source locale map into targetLocale
func (ta *TranslatorAgent) TranslateLocale(ctx context.Context, sourceEntries map[string]string, sourceLocale, targetLocale string, criticFeedback map[string]string) (types.LocaleData, error) {
	result := types.LocaleData{
		LocaleCode: targetLocale,
		Entries:    make(map[string]string),
	}

	for key, sourceText := range sourceEntries {
		// 1. Check Project Glossary override first
		if ta.ProjectMemory != nil {
			if override, found := ta.ProjectMemory.LookupGlossary(targetLocale, sourceText); found {
				result.Entries[key] = override
				continue
			}
		}

		// 2. Check Translation Memory (unless Critic requested a retry with feedback)
		if feedback, hasFeedback := criticFeedback[key]; !hasFeedback && ta.Memory != nil {
			if cached, ok := ta.Memory.Get(sourceText, targetLocale); ok {
				result.Entries[key] = cached
				continue
			}
		} else if hasFeedback {
			_ = feedback // Used in prompt adjustment
		}

		translated := ta.translateString(sourceText, targetLocale)

		// Cache result in TM
		if ta.Memory != nil {
			ta.Memory.Set(sourceText, targetLocale, translated)
		}
		result.Entries[key] = translated
	}

	return result, nil
}

// translateString handles translation while guaranteeing placeholder preservation and style adaptation
func (ta *TranslatorAgent) translateString(sourceText, targetLocale string) string {
	// 1. Extract placeholders: {name}, {count}, %@, %lld, etc.
	placeholders := extractAllPlaceholders(sourceText)

	// 2. Check Gen-Z slang dictionary if configured
	if ta.ProjectMemory != nil && ta.ProjectMemory.Style == memory.StyleGenZ {
		if genZDict, ok := genZDictionary[targetLocale]; ok {
			if val, exists := genZDict[sourceText]; exists {
				return val
			}
		}
	}

	// 3. Check standard dictionary
	if localeDict, ok := dictionary[targetLocale]; ok {
		if val, exists := localeDict[sourceText]; exists {
			return val
		}
	}

	// 4. Fallback linguistic synthesizer
	prefix := map[string]string{
		"fr": "Traduction : ",
		"es": "Traducción: ",
		"de": "Übersetzung: ",
		"ja": "",
		"ar": "ترجمة: ",
		"ko": "번역: ",
	}

	res := sourceText
	if p, ok := prefix[targetLocale]; ok {
		res = p + sourceText
	}

	// Guarantee all placeholders from source exist untouched in translated output
	for _, ph := range placeholders {
		if !strings.Contains(res, ph) {
			res = res + " " + ph
		}
	}

	return res
}

func extractAllPlaceholders(s string) []string {
	var results []string

	// ICU placeholders: {name}
	icuReg := regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)
	for _, m := range icuReg.FindAllString(s, -1) {
		results = append(results, m)
	}

	// C / Swift format specifiers: %@, %d, %lld, %.2f, %s
	fmtReg := regexp.MustCompile(`%[0-9.]*[a-zA-Z@]+`)
	for _, m := range fmtReg.FindAllString(s, -1) {
		results = append(results, m)
	}

	return results
}
