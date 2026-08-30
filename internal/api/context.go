package api

import "context"

type contextKey int

const teamIDKey contextKey = 1

func contextWithTeamID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, teamIDKey, id)
}

func teamIDFromCtx(r interface{ Context() context.Context }) int64 {
	id, _ := r.Context().Value(teamIDKey).(int64)
	return id
}
