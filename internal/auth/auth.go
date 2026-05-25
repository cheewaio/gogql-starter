package auth

import "context"

type User struct {
	Username string
}

type contextKey struct{}

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(contextKey{}).(*User)
	return user, ok
}
