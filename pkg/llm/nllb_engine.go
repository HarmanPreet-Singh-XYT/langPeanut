package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/langPeanut/langPeanut/pkg/logger"
)

// NLLBEngine encapsulates both Cloud and Local Meta NLLB-200 translation pipelines
type NLLBEngine struct {
	isLocal  bool
	endpoint string
	apiKey   string
	model    string
	http     *http.Client
}

// NewNLLBCloudEngine initializes Hugging Face hosted NLLB-200 inference
func NewNLLBCloudEngine(apiKey string) *NLLBEngine {
	if apiKey == "" {
		apiKey = os.Getenv("HF_TOKEN")
		if apiKey == "" {
			apiKey = os.Getenv("HUGGINGFACE_API_KEY")
		}
	}
	endpoint := os.Getenv("HF_INFERENCE_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api-inference.huggingface.co/models/facebook/nllb-200-distilled-600M"
	}

	return &NLLBEngine{
		isLocal:  false,
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    "facebook/nllb-200-distilled-600M",
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// NewNLLBLocalEngine initializes local offline NLLB-200 execution
func NewNLLBLocalEngine(endpoint string) *NLLBEngine {
	if endpoint == "" {
		endpoint = os.Getenv("NLLB_LOCAL_ENDPOINT")
		if endpoint == "" {
			endpoint = "http://localhost:8000/translate"
		}
	}

	return &NLLBEngine{
		isLocal:  true,
		endpoint: endpoint,
		model:    "facebook/nllb-200-distilled-600M-local",
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// TranslateStringsBatch translates a map of key -> sourceText to targetLocale with placeholder protection and automatic sentence chunking
func (e *NLLBEngine) TranslateStringsBatch(ctx context.Context, sourceStrings map[string]string, sourceLocale, targetLocale string) (map[string]string, error) {
	if len(sourceStrings) == 0 {
		return map[string]string{}, nil
	}

	srcFlores := ToFlores200(sourceLocale)
	tgtFlores := ToFlores200(targetLocale)

	// Step 1: Extract, mask placeholders, and chunk any strings exceeding safe token limits
	type itemMeta struct {
		key          string
		isChunked    bool
		chunkCount   int
		placeholders []string
	}

	var flatTextsToTranslate []string
	var items []itemMeta

	for k, src := range sourceStrings {
		masked, phs := MaskPlaceholders(src)
		chunks := SplitIntoSafeChunks(masked, MaxSafeNLLBTokenCount)

		if len(chunks) > 1 {
			items = append(items, itemMeta{
				key:          k,
				isChunked:    true,
				chunkCount:   len(chunks),
				placeholders: phs,
			})
			flatTextsToTranslate = append(flatTextsToTranslate, chunks...)
		} else {
			items = append(items, itemMeta{
				key:          k,
				isChunked:    false,
				chunkCount:   1,
				placeholders: phs,
			})
			flatTextsToTranslate = append(flatTextsToTranslate, masked)
		}
	}

	var translatedTexts []string
	var err error

	logger.Get().Info("MODEL:NLLB", fmt.Sprintf("Translating %d string items from %s to %s (isLocal=%v)", len(sourceStrings), srcFlores, tgtFlores, e.isLocal))

	// Step 2: Execute batch translation via Cloud or Local
	if !e.isLocal {
		translatedTexts, err = e.translateCloudHF(ctx, flatTextsToTranslate, srcFlores, tgtFlores)
	} else {
		translatedTexts, err = e.translateLocal(ctx, flatTextsToTranslate, srcFlores, tgtFlores)
	}

	if err != nil {
		logger.Get().Error("MODEL:NLLB", fmt.Sprintf("NLLB translation failed for %s -> %s: %v", srcFlores, tgtFlores, err), err)
		return nil, err
	}

	// Step 3: Reassemble chunked sentences, unmask placeholders, and build final map
	result := make(map[string]string, len(items))
	cursor := 0

	for _, it := range items {
		var combinedTrans string
		if it.isChunked {
			var chunkParts []string
			for c := 0; c < it.chunkCount; c++ {
				if cursor < len(translatedTexts) {
					chunkParts = append(chunkParts, translatedTexts[cursor])
				}
				cursor++
			}
			combinedTrans = strings.Join(chunkParts, " ")
		} else {
			if cursor < len(translatedTexts) {
				combinedTrans = translatedTexts[cursor]
			}
			cursor++
		}

		unmasked := UnmaskPlaceholders(combinedTrans, it.placeholders)
		result[it.key] = unmasked
	}

	logger.Get().Success("MODEL:NLLB", fmt.Sprintf("Successfully translated and reassembled %d items into %s", len(result), tgtFlores))
	return result, nil
}

func (e *NLLBEngine) translateCloudHF(ctx context.Context, texts []string, srcFlores, tgtFlores string) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	const batchSize = 15
	var allOutputs []string

	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		batchOut, err := e.translateCloudHFBatch(ctx, batch, srcFlores, tgtFlores)
		if err != nil {
			return nil, err
		}
		allOutputs = append(allOutputs, batchOut...)
	}

	return allOutputs, nil
}

func (e *NLLBEngine) translateCloudHFBatch(ctx context.Context, texts []string, srcFlores, tgtFlores string) ([]string, error) {
	endpoints := []string{
		e.endpoint,
		"https://router.huggingface.co/hf-inference/models/facebook/nllb-200-distilled-600M",
		"https://api-inference.huggingface.co/models/facebook/nllb-200-distilled-600M",
	}

	reqBody := map[string]any{
		"inputs": texts,
		"parameters": map[string]string{
			"src_lang": srcFlores,
			"tgt_lang": tgtFlores,
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, ep := range endpoints {
		if ep == "" {
			continue
		}

		for attempt := 0; attempt < 3; attempt++ {
			req, err := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewBuffer(data))
			if err != nil {
				lastErr = err
				break
			}

			req.Header.Set("Content-Type", "application/json")
			if e.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+e.apiKey)
			}

			startT := time.Now()
			resp, err := e.http.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("Hugging Face request to %s failed: %w", ep, err)
				logger.Get().Warn("MODEL:NLLB", fmt.Sprintf("Request failed to endpoint %s: %v", ep, err))
				break
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				break
			}

			if resp.StatusCode == http.StatusOK {
				logger.Get().Debug("MODEL:NLLB", fmt.Sprintf("Received 200 OK from %s in %v (%d items)", ep, time.Since(startT), len(texts)))
				// Parse Hugging Face response format: [{"translation_text": "..."}] or [[{"translation_text": "..."}]]
				var listResp []struct {
					TranslationText string `json:"translation_text"`
				}

				if err := json.Unmarshal(body, &listResp); err == nil && len(listResp) > 0 {
					out := make([]string, len(listResp))
					for i, item := range listResp {
						out[i] = item.TranslationText
					}
					return out, nil
				}

				var nestedResp [][]struct {
					TranslationText string `json:"translation_text"`
				}
				if err := json.Unmarshal(body, &nestedResp); err == nil && len(nestedResp) > 0 {
					out := make([]string, len(nestedResp))
					for i, sub := range nestedResp {
						if len(sub) > 0 {
							out[i] = sub[0].TranslationText
						}
					}
					return out, nil
				}

				return nil, fmt.Errorf("unexpected Hugging Face response structure: %s", string(body))
			}

			// Handle 503 Model Loading / Warm-up
			if resp.StatusCode == http.StatusServiceUnavailable {
				var loadErr struct {
					Error         string  `json:"error"`
					EstimatedTime float64 `json:"estimated_time"`
				}
				_ = json.Unmarshal(body, &loadErr)
				waitTime := 5 * time.Second
				if loadErr.EstimatedTime > 0 && loadErr.EstimatedTime < 20 {
					waitTime = time.Duration(loadErr.EstimatedTime) * time.Second
				}
				logger.Get().Warn("MODEL:NLLB", fmt.Sprintf("Hugging Face serverless model cold-starting (503). Retrying in %v (attempt %d/3)...", waitTime, attempt+1))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(waitTime):
					continue
				}
			}

			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				err := fmt.Errorf("Hugging Face authentication error (status %d): Please set a valid HF_TOKEN or HUGGINGFACE_API_KEY environment variable. Details: %s", resp.StatusCode, string(body))
				logger.Get().Error("MODEL:NLLB", "Authentication failure", err)
				return nil, err
			}

			lastErr = fmt.Errorf("Hugging Face API error from %s (status %d): %s", ep, resp.StatusCode, string(body))
			logger.Get().Warn("MODEL:NLLB", fmt.Sprintf("API returned status %d from %s: %s", resp.StatusCode, ep, string(body)))
			break
		}
	}

	return nil, lastErr
}

func findLlamaCLI() string {
	candidates := []string{
		"llama-cli",
		"llama",
		"/opt/homebrew/bin/llama-cli",
		"/usr/local/bin/llama-cli",
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

func (e *NLLBEngine) translateLocal(ctx context.Context, texts []string, srcFlores, tgtFlores string) ([]string, error) {
	// 1. Try local micro-endpoint if active (e.g. a CTranslate2 / ctranslate2-nllb server at localhost:8000)
	reqBody := map[string]any{
		"texts":    texts,
		"src_lang": srcFlores,
		"tgt_lang": tgtFlores,
	}

	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", e.endpoint, bytes.NewBuffer(data))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		resp, reqErr := e.http.Do(req)
		if reqErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var localRes struct {
					Translations []string `json:"translations"`
				}
				if json.Unmarshal(body, &localRes) == nil && len(localRes.Translations) == len(texts) {
					logger.Get().Success("MODEL:NLLB", fmt.Sprintf("Executed local CTranslate2 server translation via %s", e.endpoint))
					return localRes.Translations, nil
				}
			}
		}
	}

	// 2. Built-in Deterministic Linguistic Synthesizer (Unit Test / Mock Fallback Mode)
	if e.endpoint == "mock" || os.Getenv("LANGPEANUT_TEST_MODE") == "1" {
		bcpTarget := FromFlores200(tgtFlores)
		out := make([]string, len(texts))
		for i, t := range texts {
			out[i] = synthesizeOfflineTranslation(t, bcpTarget)
		}
		return out, nil
	}

	// 3. NLLB-200 mBART Architecture — requires CTranslate2, NOT llama.cpp
	//    llama.cpp only supports GPT-family (LLaMA, Mistral, Qwen). It cannot run mBART/NLLB.
	//    To run NLLB-200 locally, you need a CTranslate2 Python server:
	//      pip install ctranslate2 starlette uvicorn
	//      Then set NLLB_LOCAL_URL=http://localhost:8000
	_ = findLlamaCLI() // llama-cli exists but cannot load mBART GGUF
	err = fmt.Errorf("NLLB-200 uses mBART architecture which is not supported by llama.cpp. " +
		"For real on-device NLLB translation, use 'Meta NLLB-200 Cloud' (Hugging Face GPU, free with HF token) " +
		"or run a local CTranslate2 server at NLLB_LOCAL_URL=http://localhost:8000. " +
		"For a fully-local GPU LLM, use 'Custom / Ollama' with models like qwen2.5:7b or llama3.2")
	logger.Get().Error("MODEL:NLLB", "mBART architecture not supported by llama.cpp runner", err)
	return nil, err
}

// Built-in multilingual phrase catalog for offline execution & deterministic fallbacks
var offlinePhraseCatalog = map[string]map[string]string{
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
	"pa": {
		"Welcome back, {name}!":         "ਵਾਪਸ ਆਉਣ 'ਤੇ ਜੀ ਆਇਆਂ ਨੂੰ, {name}!",
		"Submit Order":                  "ਆਰਡਰ ਦਰਜ ਕਰੋ",
		"Cancel":                        "ਰੱਦ ਕਰੋ",
		"Search":                        "ਖੋਜੋ",
		"Book":                          "ਬੁੱਕ ਕਰੋ",
		"Settings":                      "ਸੈਟਿੰਗਾਂ",
		"Profile":                       "ਪ੍ਰੋਫਾਈਲ",
		"Notifications":                 "ਸੂਚਨਾਵਾਂ",
		"Save":                          "ਸੰਭਾਲੋ",
		"Delete":                        "ਮਿਟਾਓ",
		"Loading...":                    "ਲੋਡ ਹੋ ਰਿਹਾ ਹੈ...",
		"Sign In":                       "ਸਾਈਨ ਇਨ ਕਰੋ",
		"Sign Out":                      "ਸਾਈਨ ਆਉਟ ਕਰੋ",
		"Checkout":                      "ਚੈੱਕਆਉਟ",
	},
	"pt": {
		"Welcome back, {name}!":         "Bem-vindo de volta, {name}!",
		"Submit Order":                  "Enviar Pedido",
		"Cancel":                        "Cancelar",
		"Search":                        "Pesquisar",
		"Book":                          "Reservar",
		"Settings":                      "Configurações",
		"Profile":                       "Perfil",
		"Notifications":                 "Notificações",
		"Save":                          "Salvar",
		"Delete":                        "Excluir",
		"Total: ${amount}":              "Total: R$ {amount}",
		"Order #{orderId}":              "Pedido nº {orderId}",
		"You have {count} new messages": "Você tem {count} novas mensagens",
		"Loading...":                    "Carregando...",
		"Sign In":                       "Entrar",
		"Sign Out":                      "Sair",
		"Checkout":                      "Finalizar Compra",
	},
	"it": {
		"Welcome back, {name}!":         "Bentornato, {name}!",
		"Submit Order":                  "Invia Ordine",
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
		"Loading...":                    "Caricamento...",
		"Sign In":                       "Accedi",
		"Sign Out":                      "Esci",
		"Checkout":                      "Cassa",
	},
	"ar": {
		"Welcome back, {name}!":         "مرحبًا بعودتك، {name}!",
		"Submit Order":                  "إرسال الطلب",
		"Cancel":                        "إلغاء",
		"Search":                        "بحث",
		"Book":                          "حجز",
		"Settings":                      "الإعدادات",
		"Profile":                       "الملف الشخصي",
		"Notifications":                 "الإشعارات",
		"Save":                          "حفظ",
		"Delete":                        "حذف",
		"Total: ${amount}":              "الإجمالي: ${amount}",
		"Order #{orderId}":              "طلب رقم #{orderId}",
		"You have {count} new messages": "لديك {count} رسائل جديدة",
		"Loading...":                    "جارٍ التحميل...",
		"Sign In":                       "تسجيل الدخول",
		"Sign Out":                      "تسجيل الخروج",
		"Checkout":                      "الدفع",
	},
}

// synthesizeOfflineTranslation provides high-quality phrase translation for standalone local testing
func synthesizeOfflineTranslation(text, targetLocale string) string {
	// Preserve masked placeholders
	re := regexp.MustCompile(`<ph_\d+/>`)
	placeholders := re.FindAllString(text, -1)

	// Clean lookup
	clean := text
	for _, ph := range placeholders {
		clean = strings.ReplaceAll(clean, ph, "{var}")
	}

	normTarget := strings.ToLower(targetLocale)
	if strings.Contains(normTarget, "-") {
		normTarget = strings.Split(normTarget, "-")[0]
	}

	if dict, ok := offlinePhraseCatalog[normTarget]; ok {
		if val, exists := dict[text]; exists {
			return val
		}
		if val, exists := dict[clean]; exists {
			return val
		}
	}

	// Dynamic word-level fallback translation map for recognized UI keywords
	var wordMap = map[string]map[string]string{
		"es": {
			"welcome": "bienvenido", "to": "a", "effortless": "sin esfuerzo", "software": "software",
			"localization": "localización", "hello": "hola", "home": "inicio", "dashboard": "panel",
			"overview": "resumen", "get started": "comenzar", "documentation": "documentación",
		},
		"fr": {
			"welcome": "bienvenue", "to": "sur", "effortless": "sans effort", "software": "logiciel",
			"localization": "localisation", "hello": "bonjour", "home": "accueil", "dashboard": "tableau de bord",
			"overview": "aperçu", "get started": "commencer", "documentation": "documentation",
		},
		"de": {
			"welcome": "willkommen", "to": "bei", "effortless": "mühelos", "software": "Software",
			"localization": "Lokalisierung", "hello": "Hallo", "home": "Startseite", "dashboard": "Übersicht",
			"overview": "Überblick", "get started": "Loslegen", "documentation": "Dokumentation",
		},
	}

	if wm, ok := wordMap[normTarget]; ok {
		res := text
		for enW, trW := range wm {
			reWord := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(enW) + `\b`)
			res = reWord.ReplaceAllString(res, trW)
		}
		if res != text {
			return res
		}
	}

	// When offline and locale is not English, produce a localized representation
	if normTarget != "en" {
		translated := fmt.Sprintf("[%s] %s", strings.ToUpper(targetLocale), text)
		for _, ph := range placeholders {
			if !strings.Contains(translated, ph) {
				translated += " " + ph
			}
		}
		return translated
	}

	return text
}
