package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/cheewaio/gogql-starter/internal/auth"
)

func runToken(username string) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
		slog.Warn("JWT_SECRET not set, using dev-secret")
	}

	token, err := auth.GenerateToken(secret, &auth.User{Username: username})
	if err != nil {
		slog.Error("generate token", "error", err)
		os.Exit(1)
	}

	fmt.Println(token)
}
