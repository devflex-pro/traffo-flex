package auth

import "context"

type contextKey string

const userContextKey contextKey = "auth_user"

func ContextWithUser(
	ctx context.Context,
	user User,
) context.Context {
	return context.WithValue(
		ctx,
		userContextKey,
		user,
	)
}

func UserFromContext(ctx context.Context) (
	User,
	bool,
) {
	user, ok := ctx.Value(userContextKey).(User)
	return user, ok
}
