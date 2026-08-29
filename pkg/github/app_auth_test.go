package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setAPIBaseURLForTest(url string) {
	apiBaseURL = url
}

func newTestCtx() context.Context {
	return context.Background()
}

func generateTestKeyPEM(t *testing.T) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	return pem.EncodeToMemory(block), key
}

func TestParsePrivateKeyPEM_PKCS1(t *testing.T) {
	pemBytes, want := generateTestKeyPEM(t)
	got, err := ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM: %v", err)
	}
	if got.N.Cmp(want.N) != 0 {
		t.Errorf("parsed key modulus mismatch")
	}
}

func TestParsePrivateKeyPEM_InvalidPEM(t *testing.T) {
	_, err := ParsePrivateKeyPEM([]byte("not a pem file"))
	if err == nil {
		t.Fatal("expected error for invalid PEM input")
	}
}

func TestSignAppJWT_StructureAndClaims(t *testing.T) {
	_, key := generateTestKeyPEM(t)
	cfg := AppConfig{AppID: "123456", PrivateKey: key}

	tok, err := signAppJWT(cfg)
	if err != nil {
		t.Fatalf("signAppJWT: %v", err)
	}

	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("expected JWT with 3 parts, got %d", len(parts))
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims["iss"] != cfg.AppID {
		t.Errorf("iss = %v, want %v", claims["iss"], cfg.AppID)
	}
	exp := int64(claims["exp"].(float64))
	iat := int64(claims["iat"].(float64))
	if exp-iat <= 0 {
		t.Errorf("exp (%d) should be after iat (%d)", exp, iat)
	}
	if time.Unix(exp, 0).Before(time.Now()) {
		t.Errorf("token should not already be expired")
	}

	// Verify signature validates against the public key.
	pub := &key.PublicKey
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	hashed := sha256Sum(signingInput)
	if err := rsa.VerifyPKCS1v15(pub, cryptoSHA256, hashed, sig); err != nil {
		t.Errorf("signature does not verify against public key: %v", err)
	}
}

func sha256Sum(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func TestCreateInstallationToken_Success(t *testing.T) {
	_, key := generateTestKeyPEM(t)
	cfg := AppConfig{AppID: "1", PrivateKey: key}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Bearer auth header, got %q", auth)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"token":"ghs_testtoken123","expires_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	restoreBase := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restoreBase)

	tok, err := CreateInstallationToken(newTestCtx(), cfg, 42)
	if err != nil {
		t.Fatalf("CreateInstallationToken: %v", err)
	}
	if tok.Token != "ghs_testtoken123" {
		t.Errorf("token = %q, want ghs_testtoken123", tok.Token)
	}
}

func TestListInstallationRepos_Paginates(t *testing.T) {
	pageCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			repos := make([]map[string]any, 100)
			for i := range repos {
				repos[i] = map[string]any{"id": i, "name": "repo", "full_name": "org/repo", "private": false, "default_branch": "main", "clone_url": "https://github.com/org/repo.git"}
			}
			json.NewEncoder(w).Encode(map[string]any{"total_count": 101, "repositories": repos})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"total_count": 101, "repositories": []map[string]any{
			{"id": 999, "name": "last-repo", "full_name": "org/last-repo", "private": true, "default_branch": "main", "clone_url": "https://github.com/org/last-repo.git"},
		}})
	}))
	defer srv.Close()

	restoreBase := apiBaseURL
	setAPIBaseURLForTest(srv.URL)
	defer setAPIBaseURLForTest(restoreBase)

	repos, err := ListInstallationRepos(newTestCtx(), "fake-token")
	if err != nil {
		t.Fatalf("ListInstallationRepos: %v", err)
	}
	if len(repos) != 101 {
		t.Errorf("expected 101 repos across pages, got %d", len(repos))
	}
	if pageCount != 2 {
		t.Errorf("expected 2 page requests, got %d", pageCount)
	}
	if repos[100].FullName != "org/last-repo" {
		t.Errorf("last repo = %q, want org/last-repo", repos[100].FullName)
	}
}
