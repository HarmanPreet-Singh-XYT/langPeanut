package auth

import (
	"testing"
)

func TestCrypto_RoundTrip(t *testing.T) {
	masterKey, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}

	secret := "sk-test-secret-key-123456789"
	ciphertext, err := Encrypt(masterKey, secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(masterKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != secret {
		t.Errorf("decrypted = %q; want %q", decrypted, secret)
	}
}

func TestCrypto_InvalidKey(t *testing.T) {
	_, err := Encrypt("short", "secret")
	if err == nil {
		t.Error("expected error with short key, got nil")
	}
}

func TestCrypto_PassphraseWithSymbols(t *testing.T) {
	masterKey := "my-secret-key-$with-dollar-signs-and-symbols-12345"
	secret := "sk-ant-api03-abcdef123456789"
	ciphertext, err := Encrypt(masterKey, secret)
	if err != nil {
		t.Fatalf("Encrypt failed with symbols in masterKey: %v", err)
	}

	decrypted, err := Decrypt(masterKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed with symbols in masterKey: %v", err)
	}

	if decrypted != secret {
		t.Errorf("decrypted = %q; want %q", decrypted, secret)
	}
}
