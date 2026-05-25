package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/cheewaio/gogql-starter/graph"
	"github.com/cheewaio/gogql-starter/internal/auth"
	"github.com/cheewaio/gogql-starter/internal/service"
	"github.com/cheewaio/gogql-starter/internal/store"
	"github.com/vektah/gqlparser/v2/ast"
)

const defaultPort = "4000"

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Info("No .env file found; continuing")
	}

	if len(os.Args) > 1 && os.Args[1] == "token" {
		username := "user@example.com"
		if len(os.Args) > 2 {
			username = os.Args[2]
		}
		runToken(username)
		return
	}

	runMigrations()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	startServer(port)
}

func runMigrations() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL not set; skipping migrations")
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		slog.Error("get wd", "error", err)
	}
	src := "file://" + filepath.ToSlash(filepath.Join(wd, "database", "migrations"))

	m, err := migrate.New(src, dbURL)
	if err != nil {
		slog.Error("migrations init", "error", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		slog.Error("migrations up", "error", err)
	}
	slog.Info("migrations applied (or none to apply)")
}

func startServer(port string) {
	slog.Info("starting server", "env", orDefault("SERVICE_ENVIRONMENT", "local"))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		slog.Error("DATABASE_URL not set; server cannot start")
		return
	}

	db, err := store.NewDB(dbURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		return
	}
	queries := store.New(db)
	noteService := service.NewNoteService(queries)

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: graph.NewResolver(noteService)}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Warn("JWT_SECRET not set; auth disabled")
	}

	graphqlHandler := auth.Middleware(jwtSecret)(srv)

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.RawQuery == "" && os.Getenv("DEBUG") == "true" {
			playground.ApolloSandboxHandler("GraphQL playground", "/graphql").ServeHTTP(w, r)
			return
		}
		graphqlHandler.ServeHTTP(w, r)
	})

	if os.Getenv("DEBUG") == "true" {
		http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/graphql", http.StatusFound)
		}))
	}

	log.Printf("connect to http://localhost:%s/graphql", port) //nolint:gosec
	httpServer := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Fatal(httpServer.ListenAndServe())
}

func orDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
