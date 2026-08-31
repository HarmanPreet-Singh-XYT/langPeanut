package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	userAgent = "langPeanut-Cloud"
	jwtTTL    = 9 * time.Minute // GitHub caps App JWTs at 10 minutes; leave margin for clock drift
)

// apiBaseURL is a var (not const) so tests can point it at an httptest server.
var apiBaseURL = "https://api.github.com"

// AppConfig holds a GitHub App's identity used to mint installation tokens.
type AppConfig struct {
	AppID      string // GitHub App ID (numeric, as string)
	PrivateKey *rsa.PrivateKey
}

// ParsePrivateKeyPEM parses a PEM-encoded RSA private key as downloaded from the
// GitHub App settings page ("Generate a private key").
func ParsePrivateKeyPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

// signAppJWT builds and signs the RS256 JWT GitHub requires to authenticate as
// the App itself (distinct from an installation). Implemented against stdlib
// crypto/rsa directly rather than pulling in a JWT library for one token shape.
func signAppJWT(cfg AppConfig) (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(), // backdate slightly for clock drift, per GitHub's docs
		"exp": now.Add(jwtTTL).Unix(),
		"iss": cfg.AppID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := base64URLEncode(headerJSON) + "." + base64URLEncode(claimsJSON)

	hashed := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, cfg.PrivateKey, cryptoSHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("sign app jwt: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// InstallationToken is a short-lived token scoped to one App installation
// (i.e. one org/user's access grant, covering the repos they selected).
type InstallationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateInstallationToken exchanges the App's JWT for a scoped installation
// access token, used for all subsequent clone/push/PR calls against repos
// under that installation.
func CreateInstallationToken(ctx httpContext, cfg AppConfig, installationID int64) (*InstallationToken, error) {
	appJWT, err := signAppJWT(cfg)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", apiBaseURL, installationID)
	req, err := newRequest(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)

	resp, err := doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create installation token: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}

	var tok InstallationToken
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decode installation token response: %w", err)
	}
	return &tok, nil
}

// Installation represents one GitHub App installation (one org or user account
// that has installed the App), as returned by GET /app/installations.
type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

// ListInstallations returns every account (org or user) that has installed
// this GitHub App — the entry point for "connect GitHub" in the web UI: after
// OAuth login, the UI resolves which installation(s) the logged-in user has
// access to and lets them pick a repo from ListInstallationRepos.
func ListInstallations(ctx httpContext, cfg AppConfig) ([]Installation, error) {
	appJWT, err := signAppJWT(cfg)
	if err != nil {
		return nil, err
	}

	req, err := newRequest(ctx, http.MethodGet, apiBaseURL+"/app/installations", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)

	resp, err := doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list installations: unexpected status %d: %s", resp.StatusCode, readBody(resp))
	}

	var installations []Installation
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return nil, fmt.Errorf("decode installations response: %w", err)
	}
	return installations, nil
}

// Repo describes one repository accessible to an installation — the shape the
// web UI's repo picker renders after a team connects GitHub.
type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
}

type listRepositoriesResponse struct {
	TotalCount   int    `json:"total_count"`
	Repositories []Repo `json:"repositories"`
}

// ListInstallationRepos returns every repo the given installation token can
// see (i.e. every repo the user granted the App access to — "all" if they
// chose all-repos during install, or the specific subset otherwise). This is
// the direct backer for "connect GitHub, then we get all the repos, and we
// can choose any repo" — the web UI calls this once per installation and
// renders the result as the pickable repo list. Paginates through all pages
// since an org can have far more than the 100-per-page default.
func ListInstallationRepos(ctx httpContext, installationToken string) ([]Repo, error) {
	var all []Repo
	page := 1
	for {
		url := fmt.Sprintf("%s/installation/repositories?per_page=100&page=%d", apiBaseURL, page)
		req, err := newRequest(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+installationToken)

		resp, err := doRequest(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body := readBody(resp)
			resp.Body.Close()
			return nil, fmt.Errorf("list installation repos: unexpected status %d: %s", resp.StatusCode, body)
		}

		var page_ listRepositoriesResponse
		err = json.NewDecoder(resp.Body).Decode(&page_)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode installation repos response: %w", err)
		}

		all = append(all, page_.Repositories...)
		if len(page_.Repositories) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return string(b)
}
