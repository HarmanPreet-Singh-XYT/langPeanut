package api

import (
	"context"

	"github.com/langPeanut/langpeanut-cloud/internal/auth"
)

type contextKey int

const sessionKey contextKey = 1

func contextWithSession(ctx context.Context, sess *auth.Session) context.Context {
	return context.WithValue(ctx, sessionKey, sess)
}

// sessionFromCtx returns the authenticated session set by requireSession.
// Only call from handlers wrapped with requireSession — it panics on a nil
// session rather than silently falling back to a guessable identity, since a
// missing session here means the middleware wiring itself is broken.
func sessionFromCtx(r interface{ Context() context.Context }) *auth.Session {
	sess, ok := r.Context().Value(sessionKey).(*auth.Session)
	if !ok || sess == nil {
		panic("api: sessionFromCtx called on a request without requireSession")
	}
	return sess
}
