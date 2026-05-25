package graph_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/cheewaio/gogql-starter/graph"
	"github.com/cheewaio/gogql-starter/internal/auth"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

func newHandler() http.Handler {
	resolver := graph.NewResolver(nil)
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})
	mux := http.NewServeMux()
	mux.Handle("/query", srv)
	return auth.Middleware("test-secret")(mux)
}

func TestNotesQuery_ReturnsError(t *testing.T) {
	token, err := auth.GenerateToken("test-secret", &auth.User{Username: "550e8400-e29b-41d4-a716-446655440000"})
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(newHandler())
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/query",
		strings.NewReader(`{"query": "{ notes { items { id content createdAt modifiedAt user { id username } } pageInfo { page pageSize total totalPages } } }"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected error but got none")
	}
}

func TestCreateNoteMutation_ReturnsError(t *testing.T) {
	token, err := auth.GenerateToken("test-secret", &auth.User{Username: "550e8400-e29b-41d4-a716-446655440000"})
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(newHandler())
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/query",
		strings.NewReader(`{"query": "mutation { createNote(input: {content: \"test\"}) { id } }"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected error but got none")
	}
}

func TestCreateNoteMutation_RejectsUnauthenticated(t *testing.T) {
	ts := httptest.NewServer(newHandler())
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/query", strings.NewReader(
		`{"query": "mutation { createNote(input: {content: \"test\"}) { id } }"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected error but got none")
	}
	if result.Errors[0].Message != "authentication required" {
		t.Errorf("unexpected error message: got %q", result.Errors[0].Message)
	}
}

func TestPlaygroundEndpoint_ReturnsOK(t *testing.T) {
	mux := http.NewServeMux()
	resolver := graph.NewResolver(nil)
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})
	mux.Handle("/playground", playground.Handler("GraphQL playground", "/graphql"))
	mux.Handle("/graphql", srv)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/playground", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
