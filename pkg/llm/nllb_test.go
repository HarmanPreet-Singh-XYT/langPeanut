package llm

import (
	"context"
	"strings"
	"testing"
)

func TestToFlores200(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"en", "eng_Latn"},
		{"en-US", "eng_Latn"},
		{"es", "spa_Latn"},
		{"fr", "fra_Latn"},
		{"de", "deu_Latn"},
		{"hi", "hin_Deva"},
		{"pa", "pan_Guru"},
		{"ja", "jpn_Jpan"},
		{"zh", "zho_Hans"},
		{"zh-CN", "zho_Hans"},
		{"zh-TW", "zho_Hant"},
		{"ar", "ara_Arab"},
		{"ru", "rus_Cyrl"},
		{"ko", "kor_Hang"},
		{"pt-BR", "por_Latn"},
		{"fra_Latn", "fra_Latn"}, // Already FLORES-200
		{"unknown-lang", "eng_Latn"},
	}

	for _, c := range cases {
		actual := ToFlores200(c.input)
		if actual != c.expected {
			t.Errorf("ToFlores200(%q) = %q, expected %q", c.input, actual, c.expected)
		}
	}
}

func TestFromFlores200(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"eng_Latn", "en"},
		{"fra_Latn", "fr"},
		{"spa_Latn", "es"},
		{"hin_Deva", "hi"},
		{"pan_Guru", "pa"},
		{"jpn_Jpan", "ja"},
	}

	for _, c := range cases {
		actual := FromFlores200(c.input)
		if actual != c.expected {
			t.Errorf("FromFlores200(%q) = %q, expected %q", c.input, actual, c.expected)
		}
	}
}

func TestMaskAndUnmaskPlaceholders(t *testing.T) {
	cases := []struct {
		original string
	}{
		{"Welcome back, {name}!"},
		{"You have {count} new notifications for {user}."},
		{"Total: ${amount} (Code: %@)"},
		{"User #{userId}: ${balance}"},
		{"Hello \\(name), your items: {items}"},
	}

	for _, c := range cases {
		masked, placeholders := MaskPlaceholders(c.original)
		if len(placeholders) == 0 {
			t.Errorf("expected placeholders in %q, found none", c.original)
		}

		// Ensure <ph_0/> format is present
		if !strings.Contains(masked, "<ph_0/>") {
			t.Errorf("expected <ph_0/> in masked string: %q", masked)
		}

		// Simulate translation where words change around placeholders
		simulatedTranslated := strings.Replace(masked, "Welcome back", "Bon retour", 1)
		simulatedTranslated = strings.Replace(simulatedTranslated, "You have", "Vous avez", 1)

		unmasked := UnmaskPlaceholders(simulatedTranslated, placeholders)

		for _, ph := range placeholders {
			if !strings.Contains(unmasked, ph) {
				t.Errorf("unmasked string %q lost placeholder %q from original %q", unmasked, ph, c.original)
			}
		}
	}
}

func TestNLLBEngine_TranslateStringsBatch(t *testing.T) {
	engine := NewNLLBLocalEngine("mock")

	sources := map[string]string{
		"welcome": "Welcome back, {name}!",
		"submit":  "Submit Order",
		"cancel":  "Cancel",
	}

	res, err := engine.TranslateStringsBatch(context.Background(), sources, "en", "fr")
	if err != nil {
		t.Fatalf("TranslateStringsBatch returned unexpected error: %v", err)
	}

	if len(res) != len(sources) {
		t.Errorf("expected %d translations, got %d", len(sources), len(res))
	}

	// Verify placeholder integrity
	if welcomeTrans, ok := res["welcome"]; !ok || !strings.Contains(welcomeTrans, "{name}") {
		t.Errorf("welcome translation %q lost {name} placeholder", welcomeTrans)
	}
}

func TestSplitIntoSafeChunks(t *testing.T) {
	shortText := "Welcome to our application."
	shortChunks := SplitIntoSafeChunks(shortText, 50)
	if len(shortChunks) != 1 {
		t.Errorf("expected 1 chunk for short text, got %d", len(shortChunks))
	}

	// Long text with multiple sentences exceeding word threshold
	longText := "First sentence here. Second sentence with details. Third sentence goes here. Fourth sentence wrapping up. Fifth sentence final note."
	chunks := SplitIntoSafeChunks(longText, 6) // Max 6 words per chunk
	if len(chunks) <= 1 {
		t.Errorf("expected multiple chunks for long text, got %d", len(chunks))
	}

	// Reassembled text should contain all sentences
	combined := strings.Join(chunks, " ")
	if !strings.Contains(combined, "First sentence") || !strings.Contains(combined, "Fifth sentence") {
		t.Errorf("reassembled text lost sentences: %q", combined)
	}
}

func TestNLLBEngine_LongTextBatchTranslation(t *testing.T) {
	engine := NewNLLBLocalEngine("mock")

	longContent := "Welcome back, {name}! We are excited to have you on board. Please check your notifications for updates. Total balance: ${amount}."
	sources := map[string]string{
		"longKey": longContent,
	}

	res, err := engine.TranslateStringsBatch(context.Background(), sources, "en", "es")
	if err != nil {
		t.Fatalf("TranslateStringsBatch returned error: %v", err)
	}

	translated := res["longKey"]
	if !strings.Contains(translated, "{name}") || !strings.Contains(translated, "${amount}") {
		t.Errorf("translated long text %q lost placeholders", translated)
	}
}

