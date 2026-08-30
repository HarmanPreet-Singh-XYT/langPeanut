// GitHub OAuth (user login) — the "user-to-server" flow for the GitHub App
// already configured for installation tokens (see langPeanut/pkg/github).
// This is a separate credential pair (Client ID/Secret) from the App ID +
// private key used to mint installation tokens; GitHub Apps carry both.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var oauthHTTPClient = &http.Client{Timeout: 15 * time.Second}

// AuthorizeURL builds the GitHub OAuth authorization redirect URL.
func AuthorizeURL(clientID, redirectURI, state string) string {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	return "https://github.com/login/oauth/authorize?" + v.Encode()
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// ExchangeCode trades an OAuth `code` for a user access token.
func ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", nil)
	if err != nil {
		return "", err
	}
	req.URL.RawQuery = form.Encode()
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	var tok oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tok.Error != "" {
		return "", fmt.Errorf("github oauth error: %s (%s)", tok.Error, tok.ErrorDesc)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("github oauth: empty access token")
	}
	return tok.AccessToken, nil
}

// GitHubUser is the subset of GET /user this app needs.
type GitHubUser struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	ID        int64  `json:"id"`
}

// FetchGitHubUser calls GET /user with the given user access token.
func FetchGitHubUser(ctx context.Context, userAccessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+userAccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("fetch github user: unexpected status %d: %s", resp.StatusCode, body)
	}

	var u GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode github user: %w", err)
	}

	// Public profile email can be empty/private; fall back to the noreply
	// address GitHub always provides so UpsertUser's UNIQUE(email) has a
	// stable, real identifier tied to this account rather than a genuinely
	// fabricated placeholder.
	if u.Email == "" {
		u.Email = fmt.Sprintf("%d+%s@users.noreply.github.com", u.ID, u.Login)
	}
	if u.Name == "" {
		u.Name = u.Login
	}
	return &u, nil
}
