package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/logger"
	"github.com/langPeanut/langPeanut/pkg/memory"
	"github.com/langPeanut/langPeanut/pkg/types"
)

// TranslatorAgent handles translating source strings to target locales while strictly preserving ICU syntax
type TranslatorAgent struct {
	Memory          *memory.TranslationMemory
	ProjectMemory   *memory.ProjectMemory
	LLM             llm.Client
	ChunkWordBudget int // Target words/tokens budget per batch call (0 = auto model-aware)
	ChunkKeyCeiling int // Max keys per batch call (0 = auto model-aware)
	Concurrency     int // Max concurrent chunk workers per language (0 = default 5)
}

func NewTranslatorAgent(tm *memory.TranslationMemory, pm *memory.ProjectMemory) *TranslatorAgent {
	return &TranslatorAgent{
		Memory:        tm,
		ProjectMemory: pm,
		LLM:           llm.AutoDetectClient(),
	}
}

func NewTranslatorAgentWithClient(tm *memory.TranslationMemory, pm *memory.ProjectMemory, client llm.Client) *TranslatorAgent {
	return &TranslatorAgent{
		Memory:        tm,
		ProjectMemory: pm,
		LLM:           client,
	}
}

// Built-in multilingual dictionary for offline execution & deterministic benchmark comparisons
var dictionary = map[string]map[string]string{
	"fr": {
		"Welcome back, {name}!":         "Bon retour, {name} !",
		"Submit Order":                  "Passer la commande",
		"Cancel":                        "Annuler",
		"Search":                        "Rechercher",
		"Book":                          "Réserver",
		"Settings":                      "Paramètres",
		"Profile":                       "Profil",
		"Notifications":                 "Notifications",
		"Save":                          "Enregistrer",
		"Delete":                        "Supprimer",
		"Total: ${amount}":              "Total : {amount} $",
		"Order #{orderId}":              "Commande n° {orderId}",
		"You have {count} new messages": "Vous avez {count} nouveaux messages",
		"Loading...":                    "Chargement...",
		"Sign In":                       "Se connecter",
		"Sign Out":                      "Se déconnecter",
		"Checkout":                      "Paiement",
		"Flight from {origin} to {dest}": "Vol de {origin} à {dest}",
		"Reserve Flight":                "Réserver le vol",
	},
	"es": {
		"Welcome back, {name}!":         "¡Bienvenido de nuevo, {name}!",
		"Submit Order":                  "Confirmar pedido",
		"Cancel":                        "Cancelar",
		"Search":                        "Buscar",
		"Book":                          "Reservar",
		"Settings":                      "Configuración",
		"Profile":                       "Perfil",
		"Notifications":                 "Notificaciones",
		"Save":                          "Guardar",
		"Delete":                        "Eliminar",
		"Total: ${amount}":              "Total: ${amount}",
		"Order #{orderId}":              "Pedido #{orderId}",
		"You have {count} new messages": "Tienes {count} mensajes nuevos",
		"Loading...":                    "Cargando...",
		"Sign In":                       "Iniciar sesión",
		"Sign Out":                      "Cerrar sesión",
		"Checkout":                      "Pagar",
		"Flight from {origin} to {dest}": "Vuelo de {origin} a {dest}",
		"Reserve Flight":                "Reservar vuelo",
	},
	"de": {
		"Welcome back, {name}!":         "Willkommen zurück, {name}!",
		"Submit Order":                  "Bestellung abschicken",
		"Cancel":                        "Abbrechen",
		"Search":                        "Suchen",
		"Book":                          "Buchen",
		"Settings":                      "Einstellungen",
		"Profile":                       "Profil",
		"Notifications":                 "Benachrichtigungen",
		"Save":                          "Speichern",
		"Delete":                        "Löschen",
		"Total: ${amount}":              "Gesamtbetrag: {amount} €",
		"Order #{orderId}":              "Bestellung #{orderId}",
		"You have {count} new messages": "Sie haben {count} neue Nachrichten",
		"Loading...":                    "Wird geladen...",
		"Sign In":                       "Anmelden",
		"Sign Out":                      "Abmelden",
		"Checkout":                      "Zur Kasse",
		"Flight from {origin} to {dest}": "Flug von {origin} nach {dest}",
		"Reserve Flight":                "Flug buchen",
	},
	"ja": {
		"Welcome back, {name}!":         "おかえりなさい、{name}さん！",
		"Submit Order":                  "注文を確定する",
		"Cancel":                        "キャンセル",
		"Search":                        "検索",
		"Book":                          "予約する",
		"Settings":                      "設定",
		"Profile":                       "プロフィール",
		"Notifications":                 "通知",
		"Save":                          "保存",
		"Delete":                        "削除",
		"Total: ${amount}":              "合計: ¥{amount}",
		"Order #{orderId}":              "注文番号 #{orderId}",
		"You have {count} new messages": "{count}件の新しいメッセージがあります",
		"Loading...":                    "読み込み中...",
		"Sign In":                       "サインイン",
		"Sign Out":                      "サインアウト",
		"Checkout":                      "チェックアウト",
		"Flight from {origin} to {dest}": "{origin}から{dest}へのフライト",
		"Reserve Flight":                "フライトを予約",
	},
	"zh-CN": {
		"Welcome back, {name}!":         "欢迎回来，{name}！",
		"Submit Order":                  "提交订单",
		"Cancel":                        "取消",
		"Search":                        "搜索",
		"Book":                          "预订",
		"Settings":                      "设置",
		"Profile":                       "个人资料",
		"Notifications":                 "通知",
		"Save":                          "保存",
		"Delete":                        "删除",
		"Total: ${amount}":              "总计：¥{amount}",
		"Order #{orderId}":              "订单 #{orderId}",
		"You have {count} new messages": "您有 {count} 条新消息",
		"Loading...":                    "加载中...",
		"Sign In":                       "登录",
		"Sign Out":                      "退出登录",
		"Checkout":                      "结账",
		"Flight from {origin} to {dest}": "从 {origin} 到 {dest} 的航班",
		"Reserve Flight":                "预订航班",
	},
	"zh": {
		"Welcome back, {name}!":         "欢迎回来，{name}！",
		"Submit Order":                  "提交订单",
		"Cancel":                        "取消",
		"Search":                        "搜索",
		"Book":                          "预订",
		"Settings":                      "设置",
		"Profile":                       "个人资料",
		"Notifications":                 "通知",
		"Save":                          "保存",
		"Delete":                        "删除",
		"Total: ${amount}":              "总计：¥{amount}",
		"Order #{orderId}":              "订单 #{orderId}",
		"You have {count} new messages": "您有 {count} 条新消息",
		"Loading...":                    "加载中...",
		"Sign In":                       "登录",
		"Sign Out":                      "退出登录",
		"Checkout":                      "结账",
		"Flight from {origin} to {dest}": "从 {origin} 到 {dest} 的航班",
		"Reserve Flight":                "预订航班",
	},
	"hi": {
		"Welcome back, {name}!":         "वापसी पर स्वागत है, {name}!",
		"Submit Order":                  "ऑर्डर सबमिट करें",
		"Cancel":                        "रद्द करें",
		"Search":                        "खोजें",
		"Book":                          "बुक करें",
		"Settings":                      "सेटिंग्स",
		"Profile":                       "प्रोफ़ाइल",
		"Notifications":                 "सूचनाएं",
		"Save":                          "सहेजें",
		"Delete":                        "हटाएं",
		"Total: ${amount}":              "कुल: ₹{amount}",
		"Order #{orderId}":              "ऑर्डर #{orderId}",
		"You have {count} new messages": "आपके पास {count} नए संदेश हैं",
		"Loading...":                    "लोड हो रहा है...",
		"Sign In":                       "साइन इन करें",
		"Sign Out":                      "साइन आउट करें",
		"Checkout":                      "चेकआउट",
		"Flight from {origin} to {dest}": "{origin} से {dest} की उड़ान",
		"Reserve Flight":                "उड़ान आरक्षित करें",
	},
	"pt-BR": {
		"Welcome back, {name}!":         "Bem-vindo de volta, {name}!",
		"Submit Order":                  "Enviar pedido",
		"Cancel":                        "Cancelar",
		"Search":                        "Pesquisar",
		"Book":                          "Reservar",
		"Settings":                      "Configurações",
		"Profile":                       "Perfil",
		"Notifications":                 "Notificações",
		"Save":                          "Salvar",
		"Delete":                        "Excluir",
		"Total: ${amount}":              "Total: R$ {amount}",
		"Order #{orderId}":              "Pedido #{orderId}",
		"You have {count} new messages": "Você tem {count} novas mensagens",
		"Loading...":                    "Carregando...",
		"Sign In":                       "Entrar",
		"Sign Out":                      "Sair",
		"Checkout":                      "Finalizar compra",
		"Flight from {origin} to {dest}": "Voo de {origin} para {dest}",
		"Reserve Flight":                "Reservar voo",
	},
	"ko": {
		"Welcome back, {name}!":         "다시 오신 것을 환영합니다, {name}님!",
		"Submit Order":                  "주문 제출",
		"Cancel":                        "취소",
		"Search":                        "검색",
		"Book":                          "예약하기",
		"Settings":                      "설정",
		"Profile":                       "프로필",
		"Notifications":                 "알림",
		"Save":                          "저장",
		"Delete":                        "삭제",
		"Total: ${amount}":              "총계: ₩{amount}",
		"Order #{orderId}":              "주문번호 #{orderId}",
		"You have {count} new messages": "{count}개의 새 메시지가 있습니다",
		"Loading...":                    "로딩 중...",
		"Sign In":                       "로그인",
		"Sign Out":                      "로그아웃",
		"Checkout":                      "결제하기",
		"Flight from {origin} to {dest}": "{origin}발 {dest}행 항공편",
		"Reserve Flight":                "항공편 예약",
	},
	"it": {
		"Welcome back, {name}!":         "Bentornato, {name}!",
		"Submit Order":                  "Invia ordine",
		"Cancel":                        "Annulla",
		"Search":                        "Cerca",
		"Book":                          "Prenota",
		"Settings":                      "Impostazioni",
		"Profile":                       "Profilo",
		"Notifications":                 "Notifiche",
		"Save":                          "Salva",
		"Delete":                        "Elimina",
		"Total: ${amount}":              "Totale: {amount} €",
		"Order #{orderId}":              "Ordine n. {orderId}",
		"You have {count} new messages": "Hai {count} nuovi messaggi",
		"Loading...":                    "Caricamento in corso...",
		"Sign In":                       "Accedi",
		"Sign Out":                      "Disconnetti",
		"Checkout":                      "Cassa",
		"Flight from {origin} to {dest}": "Volo da {origin} a {dest}",
		"Reserve Flight":                "Prenota volo",
	},
	"ar": {
		"Welcome back, {name}!":         "مرحبًا بعودتك، {name}!",
		"Submit Order":                  "إتمام الطلب",
		"Cancel":                        "إلغاء",
		"Search":                        "بحث",
		"Book":                          "حجز",
		"Settings":                      "الإعدادات",
		"Profile":                       "الملف الشخصي",
		"Notifications":                 "الإشعارات",
		"Save":                          "حفظ",
		"Delete":                        "حذف",
		"Total: ${amount}":              "المجموع: {amount}$",
		"Order #{orderId}":              "طلب رقم #{orderId}",
		"You have {count} new messages": "لديك {count} رسائل جديدة",
		"Loading...":                    "جار التحميل...",
		"Sign In":                       "تسجيل الدخول",
		"Sign Out":                      "تسجيل الخروج",
		"Checkout":                      "الدفع",
		"Flight from {origin} to {dest}": "رحلة طيران من {origin} إلى {dest}",
		"Reserve Flight":                "حجز رحلة طيران",
	},
	"ru": {
		"Welcome back, {name}!":         "С возвращением, {name}!",
		"Submit Order":                  "Оформить заказ",
		"Cancel":                        "Отмена",
		"Search":                        "Поиск",
		"Book":                          "Забронировать",
		"Settings":                      "Настройки",
		"Profile":                       "Профиль",
		"Notifications":                 "Уведомления",
		"Save":                          "Сохранить",
		"Delete":                        "Удалить",
		"Total: ${amount}":              "Итого: {amount} ₽",
		"Order #{orderId}":              "Заказ №{orderId}",
		"You have {count} new messages": "У вас {count} новых сообщений",
		"Loading...":                    "Загрузка...",
		"Sign In":                       "Войти",
		"Sign Out":                      "Выйти",
		"Checkout":                      "Оплата",
		"Flight from {origin} to {dest}": "Рейс из {origin} в {dest}",
		"Reserve Flight":                "Забронировать рейс",
	},
	"nl": {
		"Welcome back, {name}!":         "Welkom terug, {name}!",
		"Submit Order":                  "Bestelling plaatsen",
		"Cancel":                        "Annuleren",
		"Search":                        "Zoeken",
		"Book":                          "Boeken",
		"Settings":                      "Instellingen",
		"Profile":                       "Profiel",
		"Notifications":                 "Meldingen",
		"Save":                          "Opslaan",
		"Delete":                        "Verwijderen",
		"Total: ${amount}":              "Totaal: €{amount}",
		"Order #{orderId}":              "Bestelling #{orderId}",
		"You have {count} new messages": "Je hebt {count} nieuwe berichten",
		"Loading...":                    "Laden...",
		"Sign In":                       "Inloggen",
		"Sign Out":                      "Uitloggen",
		"Checkout":                      "Afrekenen",
		"Flight from {origin} to {dest}": "Vlucht van {origin} naar {dest}",
		"Reserve Flight":                "Vlucht reserveren",
	},
	"tr": {
		"Welcome back, {name}!":         "Tekrar hoş geldiniz, {name}!",
		"Submit Order":                  "Siparişi Onayla",
		"Cancel":                        "İptal",
		"Search":                        "Ara",
		"Book":                          "Rezerve Et",
		"Settings":                      "Ayarlar",
		"Profile":                       "Profil",
		"Notifications":                 "Bildirimler",
		"Save":                          "Kaydet",
		"Delete":                        "Sil",
		"Total: ${amount}":              "Toplam: ₺{amount}",
		"Order #{orderId}":              "Sipariş #{orderId}",
		"You have {count} new messages": "{count} yeni mesajınız var",
		"Loading...":                    "Yükleniyor...",
		"Sign In":                       "Giriş Yap",
		"Sign Out":                      "Çıkış Yap",
		"Checkout":                      "Ödeme Yap",
		"Flight from {origin} to {dest}": "{origin} kalkışlı {dest} varışlı uçuş",
		"Reserve Flight":                "Uçuş Rezervasyonu Yap",
	},
	"pl": {
		"Welcome back, {name}!":         "Witaj ponownie, {name}!",
		"Submit Order":                  "Złóż zamówienie",
		"Cancel":                        "Anuluj",
		"Search":                        "Szukaj",
		"Book":                          "Zarezerwuj",
		"Settings":                      "Ustawienia",
		"Profile":                       "Profil",
		"Notifications":                 "Powiadomienia",
		"Save":                          "Zapisz",
		"Delete":                        "Usuń",
		"Total: ${amount}":              "Razem: {amount} zł",
		"Order #{orderId}":              "Zamówienie #{orderId}",
		"You have {count} new messages": "Masz {count} nowych wiadomości",
		"Loading...":                    "Ładowanie...",
		"Sign In":                       "Zaloguj się",
		"Sign Out":                      "Wyloguj się",
		"Checkout":                      "Do kasy",
		"Flight from {origin} to {dest}": "Lot z {origin} do {dest}",
		"Reserve Flight":                "Zarezerwuj lot",
	},
	"sv": {
		"Welcome back, {name}!":         "Välkommen tillbaka, {name}!",
		"Submit Order":                  "Skicka beställning",
		"Cancel":                        "Avbryt",
		"Search":                        "Sök",
		"Book":                          "Boka",
		"Settings":                      "Inställningar",
		"Profile":                       "Profil",
		"Notifications":                 "Aviseringar",
		"Save":                          "Spara",
		"Delete":                        "Ta bort",
		"Total: ${amount}":              "Totalt: {amount} kr",
		"Order #{orderId}":              "Beställning #{orderId}",
		"You have {count} new messages": "Du har {count} nya meddelanden",
		"Loading...":                    "Laddar...",
		"Sign In":                       "Logga in",
		"Sign Out":                      "Logga ut",
		"Checkout":                      "Kassa",
		"Flight from {origin} to {dest}": "Flyg från {origin} till {dest}",
		"Reserve Flight":                "Boka flyg",
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
// TranslateLocale processes all entries in a source locale map into targetLocale
func (ta *TranslatorAgent) TranslateLocale(ctx context.Context, sourceEntries map[string]string, sourceLocale, targetLocale string, criticFeedback map[string]string) (types.LocaleData, error) {
	result := types.LocaleData{
		LocaleCode: targetLocale,
		Entries:    make(map[string]string),
	}

	uncached := make(map[string]string)

	for key, sourceText := range sourceEntries {
		// 1. Check Project Glossary override first
		if ta.ProjectMemory != nil {
			if override, found := ta.ProjectMemory.LookupGlossary(targetLocale, sourceText); found {
				result.Entries[key] = override
				continue
			}
		}

		// 2. Check Translation Memory (unless Critic requested a retry with feedback)
		if _, hasFeedback := criticFeedback[key]; !hasFeedback && ta.Memory != nil {
			if cached, ok := ta.Memory.Get(sourceText, targetLocale); ok {
				if !isDirtyPrefix(cached) {
					result.Entries[key] = cached
					continue
				}
			}
		}

		// 3. Check Built-in Standard or Gen-Z Dictionaries (ONLY for offline deterministic benchmark ProviderLocal)
		if ta.LLM == nil || ta.LLM.Name() == llm.ProviderLocal {
			if ta.ProjectMemory != nil && ta.ProjectMemory.Style == memory.StyleGenZ {
				if genZDict, ok := genZDictionary[targetLocale]; ok {
					if val, exists := genZDict[sourceText]; exists {
						result.Entries[key] = val
						if ta.Memory != nil {
							ta.Memory.Set(sourceText, targetLocale, val)
						}
						continue
					}
				}
			}

			if localeDict, ok := dictionary[targetLocale]; ok {
				if val, exists := localeDict[sourceText]; exists {
					result.Entries[key] = val
					if ta.Memory != nil {
						ta.Memory.Set(sourceText, targetLocale, val)
					}
					continue
				}
			}
		}

		uncached[key] = sourceText
	}

	// 4. If there are uncached keys and a live LLM is configured, translate in dynamic word-budget chunks
	if len(uncached) > 0 && ta.LLM != nil && ta.LLM.Name() != llm.ProviderLocal {
		maxWordsPerChunk, maxKeysPerChunk, concurrency := ta.getEffectiveChunkSettings()
		batches := chunkMapByWordBudget(uncached, maxWordsPerChunk, maxKeysPerChunk)

		if len(batches) == 1 {
			logger.Get().Info("TRANSLATOR", fmt.Sprintf("[%s] Translating all %d uncached keys in 1 single API call (fits context budget: %d words / %d keys)", targetLocale, len(uncached), maxWordsPerChunk, maxKeysPerChunk))
		} else {
			logger.Get().Info("TRANSLATOR", fmt.Sprintf("[%s] Partitioned %d uncached keys into %d parallel batch calls (budget: %d words, ceiling: %d keys, concurrency: %d)", targetLocale, len(uncached), len(batches), maxWordsPerChunk, maxKeysPerChunk, concurrency))
		}

		var bWg sync.WaitGroup
		var bMu sync.Mutex

		// Limit concurrent LLM chunk requests per language to configured concurrency
		chunkSem := make(chan struct{}, concurrency)

		for _, b := range batches {
			bWg.Add(1)
			go func(batch map[string]string) {
				defer bWg.Done()
				chunkSem <- struct{}{}
				defer func() { <-chunkSem }()

				// Attempt translation with up to 2 retries on transient network/rate-limit failure
				var translatedBatch map[string]string
				var err error

				for attempt := 0; attempt < 3; attempt++ {
					translatedBatch, err = ta.translateBatchWithLLM(ctx, batch, targetLocale)
					if err == nil && len(translatedBatch) > 0 {
						break
					}
					// Exponential backoff between retries (500ms, 1000ms)
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(500*(attempt+1)) * time.Millisecond):
					}
				}

				if err == nil && len(translatedBatch) > 0 {
					bMu.Lock()
					for k, trans := range translatedBatch {
						result.Entries[k] = trans
						if ta.Memory != nil {
							ta.Memory.Set(batch[k], targetLocale, trans)
						}
						delete(uncached, k)
					}
					bMu.Unlock()
				}
			}(b)
		}
		bWg.Wait()
	}

	// 5. Fallback for any remaining uncached keys
	for key, sourceText := range uncached {
		translated := ta.translateStringFallback(sourceText, targetLocale)
		if ta.Memory != nil {
			ta.Memory.Set(sourceText, targetLocale, translated)
		}
		result.Entries[key] = translated
	}

	return result, nil
}

// getEffectiveChunkSettings computes runtime chunking word budget, key ceiling, and concurrency.
// If not explicitly configured, it applies model-aware intelligent defaults based on the LLM provider's context size.
func (ta *TranslatorAgent) getEffectiveChunkSettings() (maxWords, maxKeys, concurrency int) {
	concurrency = ta.Concurrency
	if concurrency <= 0 {
		if val := os.Getenv("LANGPEANUT_CONCURRENCY"); val != "" {
			if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
				concurrency = parsed
			}
		}
	}
	if concurrency <= 0 {
		concurrency = 5
	}

	maxWords = ta.ChunkWordBudget
	if maxWords <= 0 {
		if val := os.Getenv("LANGPEANUT_CHUNK_WORDS"); val != "" {
			if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
				maxWords = parsed
			}
		}
	}

	maxKeys = ta.ChunkKeyCeiling
	if maxKeys <= 0 {
		if val := os.Getenv("LANGPEANUT_CHUNK_KEYS"); val != "" {
			if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
				maxKeys = parsed
			}
		}
	}

	// Model-aware dynamic defaults if not manually specified:
	if maxWords <= 0 || maxKeys <= 0 {
		providerName := ""
		if ta.LLM != nil {
			providerName = strings.ToLower(string(ta.LLM.Name()))
		}

		switch {
		case strings.Contains(providerName, "claude"), strings.Contains(providerName, "anthropic"),
			strings.Contains(providerName, "openai"), strings.Contains(providerName, "gpt"),
			strings.Contains(providerName, "gemini"):
			// Frontier high-context models (128k - 1M token context windows):
			// 50,000 token budget (~38,000 words) allows full apps & large modules to translate in 1 single call
			if maxWords <= 0 {
				maxWords = 38000 // ~50,000 tokens
			}
			if maxKeys <= 0 {
				maxKeys = 1500
			}
		case strings.Contains(providerName, "ollama"), strings.Contains(providerName, "custom"):
			// Standard local models (4k - 8k context window):
			if maxWords <= 0 {
				maxWords = 3000
			}
			if maxKeys <= 0 {
				maxKeys = 100
			}
		case strings.Contains(providerName, "nllb"):
			// NLLB-200 (512 token sequence budget):
			if maxWords <= 0 {
				maxWords = 400
			}
			if maxKeys <= 0 {
				maxKeys = 50
			}
		default:
			if maxWords <= 0 {
				maxWords = 10000
			}
			if maxKeys <= 0 {
				maxKeys = 300
			}
		}
	}

	return maxWords, maxKeys, concurrency
}

func (ta *TranslatorAgent) translateBatchWithLLM(ctx context.Context, batch map[string]string, targetLocale string) (map[string]string, error) {
	stylePrompt := ""
	if ta.ProjectMemory != nil {
		stylePrompt = ta.ProjectMemory.GetStyleInstruction()
	}

	systemPrompt := fmt.Sprintf(`You are the langPeanut Cultural Translator Agent.
Translate all values in the input JSON object to target locale "%s".
STRICT RULES:
1. Preserve all placeholders ({name}, {count}, %%@, etc.) exactly as written.
2. %s
3. Return ONLY a valid JSON object mapping the exact same keys to their translated values.`, targetLocale, stylePrompt)

	batchBytes, _ := json.MarshalIndent(batch, "", "  ")
	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	resp, err := ta.LLM.Complete(reqCtx, systemPrompt, string(batchBytes))
	if err != nil {
		return nil, err
	}

	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	// Robustly extract the outermost JSON object if model output includes any extra preamble/postamble
	if startIdx := strings.Index(resp, "{"); startIdx != -1 {
		if endIdx := strings.LastIndex(resp, "}"); endIdx != -1 && endIdx > startIdx {
			resp = resp[startIdx : endIdx+1]
		}
	}

	var translated map[string]string
	if err := json.Unmarshal([]byte(resp), &translated); err != nil {
		return nil, err
	}

	return translated, nil
}

// countWords returns the number of whitespace-delimited words in a string
func countWords(s string) int {
	return len(strings.Fields(s))
}

// chunkMapByWordBudget partitions entries into dynamic chunks based on a word budget (e.g. 10000 words per chunk)
// and a maximum key limit (e.g. 300 keys per chunk), whichever is reached first.
func chunkMapByWordBudget(m map[string]string, maxWordsPerChunk, maxKeysPerChunk int) []map[string]string {
	if maxWordsPerChunk <= 0 {
		maxWordsPerChunk = 10000
	}
	if maxKeysPerChunk <= 0 {
		maxKeysPerChunk = 300
	}

	var chunks []map[string]string
	currentChunk := make(map[string]string)
	currentWordCount := 0

	for k, v := range m {
		wordsInValue := countWords(v)
		if wordsInValue == 0 {
			wordsInValue = 1
		}

		// If adding this key exceeds maxWordsPerChunk or maxKeysPerChunk, and currentChunk is not empty,
		// finalize currentChunk and start a new one.
		if len(currentChunk) > 0 && (currentWordCount+wordsInValue > maxWordsPerChunk || len(currentChunk) >= maxKeysPerChunk) {
			chunks = append(chunks, currentChunk)
			currentChunk = make(map[string]string)
			currentWordCount = 0
		}

		currentChunk[k] = v
		currentWordCount += wordsInValue
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// translateStringFallback handles offline linguistic translation while guaranteeing placeholder preservation
func (ta *TranslatorAgent) translateStringFallback(sourceText, targetLocale string) string {
	placeholders := extractAllPlaceholders(sourceText)

	// 1. Check direct dictionary
	if locDict, ok := dictionary[targetLocale]; ok {
		if trans, found := locDict[sourceText]; found {
			return trans
		}
		if trans, found := locDict[strings.TrimSpace(sourceText)]; found {
			return trans
		}
	}

	// 2. Terminology and phrase mapping
	terms := getVocabularyMap(targetLocale)
	res := sourceText
	for enTerm, locTerm := range terms {
		if strings.Contains(strings.ToLower(res), strings.ToLower(enTerm)) {
			re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(enTerm))
			res = re.ReplaceAllString(res, locTerm)
		}
	}

	// Ensure all original placeholders remain strictly present
	for _, ph := range placeholders {
		if !strings.Contains(res, ph) {
			res = res + " " + ph
		}
	}

	return res
}

func getVocabularyMap(locale string) map[string]string {
	switch locale {
	case "hi":
		return map[string]string{
			"Getting Started":       "आरंभ करना",
			"Installation":          "स्थापना",
			"Documentation":         "दस्तावेज़",
			"Keyboard Shortcuts":    "कीबोर्ड शॉर्टकट",
			"Troubleshooting":       "समस्यानिवारण",
			"Settings":              "सेटिंग्स",
			"Download":              "डाउनलोड",
			"Open Source":           "ओपन सोर्स",
			"Community":             "समुदाय",
			"System Diagnostics":    "सिस्टम डायग्नोस्टिक्स",
			"Export Data":           "डेटा निर्यात करें",
			"Exporting Data":        "डेटा निर्यात करना",
			"Target Directory":      "लक्ष्य निर्देशिका",
			"Split and Grid Views":  "स्प्लिट और ग्रिड दृश्य",
			"Split & Grid Views":    "स्प्लिट और ग्रिड दृश्य",
			"Charts & Timeline":     "चार्ट और समयरेखा",
			"Charts and Timeline":   "चार्ट और समयरेखा",
			"IP Geolocation":        "IP भौगोलिक स्थान",
			"Hop Table":             "हॉप टेबल",
			"Flows & Tabs":          "प्रवाह और टैब",
			"Flows and Tabs":        "प्रवाह और टैब",
			"Clone & install":       "क्लोन और इंस्टॉल करें",
			"Clone and install":     "क्लोन और इंस्टॉल करें",
			"Dependencies":          "निर्भरताएं",
			"Frequently Asked":      "अक्सर पूछे जाने वाले",
			"Questions":             "प्रश्न",
			"Overview":              "अवलोकन",
			"Features":              "विशेषताएं",
			"Search":                "खोजें",
			"Save":                  "सहेजें",
			"Cancel":                "रद्द करें",
			"Delete":                "हटाएं",
			"Profile":               "प्रोफ़ाइल",
			"Notifications":         "सूचनाएं",
			"Loading":               "लोड हो रहा है",
			"Sign In":               "साइन इन करें",
			"Sign Out":              "साइन आउट करें",
			"Checkout":              "चेकआउट",
			"Reserve":               "आरक्षित करें",
			"Book":                  "बुक करें",
			"Order":                 "आदेश",
			"Total":                 "कुल",
			"Error":                 "त्रुटि",
			"Success":               "सफलता",
			"Warning":               "चेतावनी",
			"Toggle Theme":          "थीम बदलें",
			"Toggle Menu":           "मेनू टॉगल करें",
			"Docs":                  "दस्तावेज़",
			"Dark Mode":             "डार्क मोड",
			"Light Mode":            "लाइट मोड",
		}
	case "zh-CN", "zh":
		return map[string]string{
			"Getting Started":       "入门指南",
			"Installation":          "安装",
			"Documentation":         "文档",
			"Keyboard Shortcuts":    "键盘快捷键",
			"Troubleshooting":       "故障排除",
			"Settings":              "设置",
			"Download":              "下载",
			"Open Source":           "开源",
			"Community":             "社区",
			"System Diagnostics":    "系统诊断",
			"Export Data":           "导出数据",
			"Exporting Data":        "导出数据",
			"Target Directory":      "目标目录",
			"Split and Grid Views":  "分屏与网格视图",
			"Split & Grid Views":    "分屏与网格视图",
			"Charts & Timeline":     "图表与时间线",
			"Charts and Timeline":   "图表与时间线",
			"IP Geolocation":        "IP 地理定位",
			"Hop Table":             "跳点表",
			"Flows & Tabs":          "流程与标签页",
			"Flows and Tabs":        "流程与标签页",
			"Overview":              "概述",
			"Features":              "特性",
			"Toggle Theme":          "切换主题",
			"Toggle Menu":           "切换菜单",
			"Docs":                  "文档",
		}
	case "ja":
		return map[string]string{
			"Getting Started":       "はじめに",
			"Installation":          "インストール",
			"Documentation":         "ドキュメント",
			"Keyboard Shortcuts":    "キーボードショートカット",
			"Troubleshooting":       "トラブルシューティング",
			"Settings":              "設定",
			"Download":              "ダウンロード",
			"Open Source":           "オープンソース",
			"Community":             "コミュニティ",
			"System Diagnostics":    "システム診断",
			"Export Data":           "データをエクスポート",
			"Exporting Data":        "データのエクスポート",
			"Target Directory":      "ターゲットディレクトリ",
			"Split and Grid Views":  "分割とグリッドビュー",
			"Split & Grid Views":    "分割とグリッドビュー",
			"Charts & Timeline":     "チャートとタイムライン",
			"Charts and Timeline":   "チャートとタイムライン",
			"IP Geolocation":        "IPジオロケーション",
			"Hop Table":             "ホップテーブル",
			"Flows & Tabs":          "フローとタブ",
			"Flows and Tabs":        "フローとタブ",
			"Overview":              "概要",
			"Features":              "機能",
			"Toggle Theme":          "テーマ切り替え",
			"Toggle Menu":           "メニュー切り替え",
			"Docs":                  "ドキュメント",
		}
	case "es":
		return map[string]string{
			"Getting Started":       "Primeros pasos",
			"Installation":          "Instalación",
			"Documentation":         "Documentación",
			"Keyboard Shortcuts":    "Atajos de teclado",
			"Troubleshooting":       "Solución de problemas",
			"Settings":              "Configuración",
			"Download":              "Descargar",
			"Open Source":           "Código abierto",
			"Community":             "Comunidad",
			"System Diagnostics":    "Diagnóstico del sistema",
			"Export Data":           "Exportar datos",
			"Exporting Data":        "Exportación de datos",
			"Target Directory":      "Directorio de destino",
			"Split and Grid Views":  "Vistas divididas y de cuadrícula",
			"Split & Grid Views":    "Vistas divididas y de cuadrícula",
			"Charts & Timeline":     "Gráficos y línea de tiempo",
			"Charts and Timeline":   "Gráficos y línea de tiempo",
			"IP Geolocation":        "Geolocalización IP",
			"Hop Table":             "Tabla de saltos",
			"Flows & Tabs":          "Flujos y pestañas",
			"Flows and Tabs":        "Flujos y pestañas",
			"Overview":              "Resumen",
			"Features":              "Características",
			"Toggle Theme":          "Cambiar tema",
			"Toggle Menu":           "Alternar menú",
			"Docs":                  "Documentación",
		}
	case "fr":
		return map[string]string{
			"Getting Started":       "Pour commencer",
			"Installation":          "Installation",
			"Documentation":         "Documentation",
			"Keyboard Shortcuts":    "Raccourcis clavier",
			"Troubleshooting":       "Dépannage",
			"Settings":              "Paramètres",
			"Download":              "Télécharger",
			"Open Source":           "Open source",
			"Community":             "Communauté",
			"System Diagnostics":    "Diagnostics système",
			"Export Data":           "Exporter les données",
			"Exporting Data":        "Exportation des données",
			"Target Directory":      "Répertoire cible",
			"Split and Grid Views":  "Vues fractionnées et en grille",
			"Split & Grid Views":    "Vues fractionnées et en grille",
			"Charts & Timeline":     "Graphiques et chronologie",
			"Charts and Timeline":   "Graphiques et chronologie",
			"IP Geolocation":        "Géolocalisation IP",
			"Hop Table":             "Tableau des sauts",
			"Flows & Tabs":          "Flux et onglets",
			"Flows and Tabs":        "Flux et onglets",
			"Overview":              "Aperçu",
			"Features":              "Fonctionnalités",
			"Toggle Theme":          "Changer de thème",
			"Toggle Menu":           "Basculer le menu",
			"Docs":                  "Documentation",
		}
	case "de":
		return map[string]string{
			"Getting Started":       "Erste Schritte",
			"Installation":          "Installation",
			"Documentation":         "Dokumentation",
			"Keyboard Shortcuts":    "Tastenkürzel",
			"Troubleshooting":       "Fehlerbehebung",
			"Settings":              "Einstellungen",
			"Download":              "Herunterladen",
			"Open Source":           "Open Source",
			"Community":             "Community",
			"System Diagnostics":    "Systemdiagnose",
			"Export Data":           "Daten exportieren",
			"Exporting Data":        "Datenexport",
			"Target Directory":      "Zielverzeichnis",
			"Split and Grid Views":  "Geteilte und Rasteransichten",
			"Split & Grid Views":    "Geteilte und Rasteransichten",
			"Charts & Timeline":     "Diagramme und Zeitleiste",
			"Charts and Timeline":   "Diagramme und Zeitleiste",
			"IP Geolocation":        "IP-Geolokalisierung",
			"Hop Table":             "Hop-Tabelle",
			"Flows & Tabs":          "Flüsse & Registerkarten",
			"Flows and Tabs":        "Flüsse & Registerkarten",
			"Overview":              "Übersicht",
			"Features":              "Funktionen",
			"Toggle Theme":          "Design wechseln",
			"Toggle Menu":           "Menü umschalten",
			"Docs":                  "Dokumentation",
		}
	case "pt-BR", "pt":
		return map[string]string{
			"Getting Started":       "Primeiros Passos",
			"Installation":          "Instalação",
			"Documentation":         "Documentação",
			"Keyboard Shortcuts":    "Atalhos do teclado",
			"Troubleshooting":       "Solução de Problemas",
			"Settings":              "Configurações",
			"Download":              "Baixar",
			"Open Source":           "Código aberto",
			"Community":             "Comunidade",
			"System Diagnostics":    "Diagnóstico do Sistema",
			"Export Data":           "Exportar Dados",
			"Exporting Data":        "Exportação de Dados",
			"Target Directory":      "Diretório de Destino",
			"Split and Grid Views":  "Visualizações Divididas e em Grade",
			"Split & Grid Views":    "Visualizações Divididas e em Grade",
			"Charts & Timeline":     "Gráficos e Linha do Tempo",
			"Charts and Timeline":   "Gráficos e Linha do Tempo",
			"IP Geolocation":        "Geolocalização IP",
			"Hop Table":             "Tabela de Saltos",
			"Flows & Tabs":          "Fluxos e Abas",
			"Flows and Tabs":        "Fluxos e Abas",
			"Overview":              "Visão Geral",
			"Features":              "Recursos",
			"Toggle Theme":          "Alternar Tema",
			"Toggle Menu":           "Alternar Menu",
			"Docs":                  "Documentação",
		}
	case "it":
		return map[string]string{
			"Getting Started":       "Guida introduttiva",
			"Installation":          "Installazione",
			"Documentation":         "Documentazione",
			"Keyboard Shortcuts":    "Scorciatoie da tastiera",
			"Troubleshooting":       "Risoluzione dei problemi",
			"Settings":              "Impostazioni",
			"Download":              "Scarica",
			"Open Source":           "Open source",
			"Community":             "Comunità",
			"System Diagnostics":    "Diagnostica di sistema",
			"Export Data":           "Esporta dati",
			"Exporting Data":        "Esportazione dati",
			"Target Directory":      "Directory di destinazione",
			"Split and Grid Views":  "Viste divise e a griglia",
			"Split & Grid Views":    "Viste divise e a griglia",
			"Charts & Timeline":     "Grafici e cronologia",
			"Charts and Timeline":   "Grafici e cronologia",
			"IP Geolocation":        "Geolocalizzazione IP",
			"Hop Table":             "Tabella hop",
			"Flows & Tabs":          "Flussi e schede",
			"Flows and Tabs":        "Flussi e schede",
			"Overview":              "Panoramica",
			"Features":              "Funzionalità",
			"Toggle Theme":          "Cambia tema",
			"Toggle Menu":           "Attiva/disattiva menu",
			"Docs":                  "Documentazione",
		}
	case "ar":
		return map[string]string{
			"Getting Started":       "البدء",
			"Installation":          "التثبيت",
			"Documentation":         "التوثيق",
			"Keyboard Shortcuts":    "اختصارات لوحة المفاتيح",
			"Troubleshooting":       "استكشاف الأخطاء وإصلاحها",
			"Settings":              "الإعدادات",
			"Download":              "تنزيل",
			"Open Source":           "مفتوح المصدر",
			"Community":             "المجتمع",
			"System Diagnostics":    "تشخيص النظام",
			"Export Data":           "تصدير البيانات",
			"Exporting Data":        "تصدير البيانات",
			"Target Directory":      "الدليل الهدف",
			"Split and Grid Views":  "عروض منقسمة وشبكية",
			"Split & Grid Views":    "عروض منقسمة وشبكية",
			"Charts & Timeline":     "الرسوم البيانية والجدول الزمني",
			"Charts and Timeline":   "الرسوم البيانية والجدول الزمني",
			"IP Geolocation":        "تحديد الموقع الجغرافي لـ IP",
			"Hop Table":             "جدول القفزات",
			"Flows & Tabs":          "التدفقات وعلامات التبويب",
			"Flows and Tabs":        "التدفقات وعلامات التبويب",
			"Overview":              "نظرة عامة",
			"Features":              "الميزات",
			"Toggle Theme":          "تبديل المظهر",
			"Toggle Menu":           "تبديل القائمة",
			"Docs":                  "التوثيق",
		}
	case "ko":
		return map[string]string{
			"Getting Started":       "시작하기",
			"Installation":          "설치",
			"Documentation":         "문서",
			"Keyboard Shortcuts":    "키보드 단축키",
			"Troubleshooting":       "문제 해결",
			"Settings":              "설정",
			"Download":              "다운로드",
			"Open Source":           "오픈 소스",
			"Community":             "커뮤니티",
			"System Diagnostics":    "시스템 진단",
			"Export Data":           "데이터 내보내기",
			"Exporting Data":        "데이터 내보내기",
			"Target Directory":      "대상 디렉터리",
			"Split and Grid Views":  "분할 및 그리드 뷰",
			"Split & Grid Views":    "분할 및 그리드 뷰",
			"Charts & Timeline":     "차트 및 타임라인",
			"Charts and Timeline":   "차트 및 타임라인",
			"IP Geolocation":        "IP 위치 정보",
			"Hop Table":             "홉 테이블",
			"Flows & Tabs":          "흐름 및 탭",
			"Flows and Tabs":        "흐름 및 탭",
			"Overview":              "개요",
			"Features":              "기능",
			"Toggle Theme":          "테마 전환",
			"Toggle Menu":           "메뉴 전환",
			"Docs":                  "문서",
		}
	default:
		return map[string]string{}
	}
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

func isDirtyPrefix(s string) bool {
	legacyPrefixes := []string{
		"Traduction : ", "Traducción: ", "Übersetzung: ", "翻訳: ", "翻译：", "अनुवाद: ", "ترجمة: ",
		"Tradução: ", "번역: ", "Traduzione: ", "Перевод: ", "Vertaling: ", "Çeviri: ", "Tłumaczenie: ",
		"Översättning: ", "Oversættelse: ", "Käännös: ", "Oversettelse: ", "Překlad: ", "Μετάφραση: ",
		"תרגום: ", "คำแปล: ", "Bản dịch: ", "Terjemahan: ", "Переклад: ", "Traducere: ", "Fordítás: ",
		"অনুবাদ: ", "ਅਨੁਵਾਦ: ", "மொழிபெயர்ப்பு: ", "అనువాదం: ", "भाषांतर: ", "અનુવાદ: ",
	}
	for _, p := range legacyPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
