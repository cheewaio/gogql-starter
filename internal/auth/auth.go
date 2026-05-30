// Package auth provides JWT-based authentication middleware for the GraphQL API.
// It defines the authenticated User type and context helpers for propagating
// user identity through the request chain.
package auth

import "context"

// User represents an authenticated user extracted from a JWT token.
type User struct {
	Username string
}

type contextKey struct{}

// ContextWithUser stores a User in the request context for downstream resolvers.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

// UserFromContext retrieves the authenticated User from the request context.
// Returns false if no user is present (unauthenticated request).
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(contextKey{}).(*User)
	return user, ok
}
