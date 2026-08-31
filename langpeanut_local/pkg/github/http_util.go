package github

import (
	"context"
	"crypto"
	"io"
	"net/http"
	"time"
)

// cryptoSHA256 is the hash algorithm identifier rsa.SignPKCS1v15 expects for RS256.
const cryptoSHA256 = crypto.SHA256

// httpContext is the minimal context type the GitHub client functions need —
// aliased so callers can pass context.Context without this package importing
// anything beyond the standard library's context contract.
type httpContext = context.Context

var httpClient = &http.Client{Timeout: 30 * time.Second}

func newRequest(ctx httpContext, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func doRequest(req *http.Request) (*http.Response, error) {
	return httpClient.Do(req)
}
