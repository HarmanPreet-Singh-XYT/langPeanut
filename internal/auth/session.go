// Session cookies: stateless, HMAC-signed tokens carrying {user_id, team_id, exp}.
// No server-side session table — verification is just a signature + expiry
// check, so there is no revocation list; logging out only clears the client's
// cookie. That's an accepted gap for now (see SESSION_SECRET rotation as the
// blunt revoke-everyone lever) rather than something this package hides.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const SessionCookieName = "langpeanut_session"

// SessionTTL is how long a session cookie remains valid after issuance.
const SessionTTL = 30 * 24 * time.Hour

// Session is the decoded, verified payload of a session cookie.
type Session struct {
	UserID int64
	TeamID int64
}

// NewSessionToken produces a signed, URL-safe token encoding userID/teamID
// with an expiry SessionTTL from now. secretHex must be a 64-char hex string
// (32 bytes), same shape as the MASTER_KEY used for credential encryption.
func NewSessionToken(secretHex string, userID, teamID int64) (string, error) {
	key, err := decodeKey(secretHex)
	if err != nil {
		return "", err
	}
	exp := time.Now().Add(SessionTTL).Unix()
	payload := make([]byte, 24)
	binary.BigEndian.PutUint64(payload[0:8], uint64(userID))
	binary.BigEndian.PutUint64(payload[8:16], uint64(teamID))
	binary.BigEndian.PutUint64(payload[16:24], uint64(exp))

	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	sig := mac.Sum(nil)

	body := base64.RawURLEncoding.EncodeToString(payload)
	sigEnc := base64.RawURLEncoding.EncodeToString(sig)
	return body + "." + sigEnc, nil
}

// ParseSessionToken verifies the signature and expiry of a token produced by
// NewSessionToken and returns the embedded identity.
func ParseSessionToken(secretHex, token string) (*Session, error) {
	key, err := decodeKey(secretHex)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed session token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payload) != 24 {
		return nil, errors.New("malformed session payload")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("malformed session signature")
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(sig, expectedSig) {
		return nil, errors.New("invalid session signature")
	}

	userID := int64(binary.BigEndian.Uint64(payload[0:8]))
	teamID := int64(binary.BigEndian.Uint64(payload[8:16]))
	exp := int64(binary.BigEndian.Uint64(payload[16:24]))
	if time.Now().Unix() > exp {
		return nil, errors.New("session expired")
	}
	return &Session{UserID: userID, TeamID: teamID}, nil
}

// GenerateState returns a random URL-safe token for the OAuth CSRF "state"
// parameter. Callers embed it in a short-lived signed cookie and compare it
// against the value GitHub echoes back on the callback.
func GenerateState(secretHex string) (string, error) {
	key, err := decodeKey(secretHex)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(nonce)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(nonce) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifyState checks a state token produced by GenerateState.
func VerifyState(secretHex, state string) bool {
	key, err := decodeKey(secretHex)
	if err != nil {
		return false
	}
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return false
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(nonce)
	expected := mac.Sum(nil)
	return hmac.Equal(sig, expected)
}
