package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type errorResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Errors: []struct {
			Message string `json:"message"`
		}{{Message: "authentication required"}},
	})
}

// isIntrospectionRequest checks whether the HTTP request contains a GraphQL
// introspection query (__schema). The request body is read and re-wound via
// io.NopCloser so downstream handlers (gqlgen) can still read it.
func isIntrospectionRequest(r *http.Request) bool {
	if r.Method == http.MethodGet {
		return strings.Contains(r.URL.RawQuery, "__schema")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	var req struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return strings.Contains(req.Query, "__schema")
}

// Middleware returns an HTTP middleware that enforces JWT authentication on
// every request. Introspection (__schema) queries bypass auth so that Apollo
// Sandbox and other GraphQL IDEs can fetch the schema immediately.
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isIntrospectionRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeUnauthorized(w)
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			user, err := ValidateToken(secret, tokenStr)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
