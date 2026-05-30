package graph

import "github.com/google/uuid"

// userID generates a deterministic UUID from a username using SHA-1 with the
// DNS namespace. This is a development convenience that avoids requiring a
// real user registration flow; in production, user IDs would come from an
// identity provider.
func userID(username string) string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(username)).String()
}
