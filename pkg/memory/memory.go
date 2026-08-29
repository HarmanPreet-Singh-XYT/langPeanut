package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// TranslationMemory manages persistent translation caching across runs and projects
type TranslationMemory struct {
	mu       sync.RWMutex
	filePath string
	entries  map[string]string // hash(sourceText:targetLocale) -> translatedText
}

func NewTranslationMemory(cacheDir string) (*TranslationMemory, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	tmFile := filepath.Join(cacheDir, "translations_memory.json")
	tm := &TranslationMemory{
		filePath: tmFile,
		entries:  make(map[string]string),
	}

	tm.load()
	return tm, nil
}

func (tm *TranslationMemory) load() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, err := os.ReadFile(tm.filePath)
	if err == nil {
		_ = json.Unmarshal(data, &tm.entries)
	}
}

func (tm *TranslationMemory) Save() error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	data, err := json.MarshalIndent(tm.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tm.filePath, data, 0644)
}

func (tm *TranslationMemory) Key(sourceText, targetLocale string) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s", sourceText, targetLocale)))
	return hex.EncodeToString(h.Sum(nil))
}

func (tm *TranslationMemory) Get(sourceText, targetLocale string) (string, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	key := tm.Key(sourceText, targetLocale)
	val, ok := tm.entries[key]
	return val, ok
}

func (tm *TranslationMemory) Set(sourceText, targetLocale, translatedText string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	key := tm.Key(sourceText, targetLocale)
	tm.entries[key] = translatedText
}

func (tm *TranslationMemory) Size() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.entries)
}
