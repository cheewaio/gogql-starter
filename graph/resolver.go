package graph

import (
	"context"

	"github.com/cheewaio/gogql-starter/internal/auth"
	"github.com/cheewaio/gogql-starter/internal/service"
)

type Resolver struct {
	NoteService *service.NoteService
}

func NewResolver(noteService *service.NoteService) *Resolver {
	return &Resolver{NoteService: noteService}
}

func (r *Resolver) GetCurrentUser(ctx context.Context) *auth.User {
	user, _ := auth.UserFromContext(ctx)
	return user
}
