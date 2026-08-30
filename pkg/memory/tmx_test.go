package memory

import (
	"strings"
	"testing"
)

func TestTMX_ExportAndImport(t *testing.T) {
	units := []TMUnit{
		{
			Key:        "welcomeTitle",
			SourceLang: "en",
			SourceText: "Welcome back, {name}!",
			TargetLang: "es",
			TargetText: "¡Bienvenido de nuevo, {name}!",
		},
		{
			Key:        "welcomeTitle",
			SourceLang: "en",
			SourceText: "Welcome back, {name}!",
			TargetLang: "de",
			TargetText: "Willkommen zurück, {name}!",
		},
		{
			Key:        "checkoutBtn",
			SourceLang: "en",
			SourceText: "Submit Order",
			TargetLang: "fr",
			TargetText: "Valider la commande",
		},
	}

	tmxBytes, err := ExportTMX(units, "en")
	if err != nil {
		t.Fatalf("ExportTMX failed: %v", err)
	}

	tmxStr := string(tmxBytes)
	if !strings.Contains(tmxStr, "<tmx version=\"1.4\">") {
		t.Errorf("Expected TMX 1.4 header, got:\n%s", tmxStr)
	}
	if !strings.Contains(tmxStr, "¡Bienvenido de nuevo") {
		t.Errorf("Expected Spanish text in TMX export")
	}

	imported, err := ImportTMX(tmxBytes)
	if err != nil {
		t.Fatalf("ImportTMX failed: %v", err)
	}

	if len(imported) != 3 {
		t.Fatalf("Expected 3 imported units, got %d", len(imported))
	}
}

func TestXLIFF_ExportAndImport(t *testing.T) {
	units := []TMUnit{
		{
			Key:        "saveAction",
			SourceLang: "en",
			SourceText: "Save Changes",
			TargetLang: "es",
			TargetText: "Guardar Cambios",
		},
		{
			Key:        "cancelAction",
			SourceLang: "en",
			SourceText: "Cancel",
			TargetLang: "es",
			TargetText: "Cancelar",
		},
	}

	xliffBytes, err := ExportXLIFF(units, "en", "es")
	if err != nil {
		t.Fatalf("ExportXLIFF failed: %v", err)
	}

	xliffStr := string(xliffBytes)
	if !strings.Contains(xliffStr, "<xliff version=\"1.2\">") {
		t.Errorf("Expected XLIFF 1.2 header, got:\n%s", xliffStr)
	}
	if !strings.Contains(xliffStr, "<target>Guardar Cambios</target>") {
		t.Errorf("Expected target element in XLIFF export")
	}

	imported, err := ImportXLIFF(xliffBytes)
	if err != nil {
		t.Fatalf("ImportXLIFF failed: %v", err)
	}

	if len(imported) != 2 {
		t.Fatalf("Expected 2 imported units, got %d", len(imported))
	}
}
