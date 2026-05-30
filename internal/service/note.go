// Package service implements application business logic on top of the store
// layer. It enforces authorization (users can only access their own notes),
// provides structured logging for audit-worthy operations, and translates
// store-level errors into user-facing messages.
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/cheewaio/gogql-starter/internal/store"
	"github.com/google/uuid"
)

// NoteService wraps store.Queries to provide business-logic methods for note
// CRUD operations with ownership checks and logging.
type NoteService struct {
	queries *store.Queries
}

// NewNoteService creates a new NoteService backed by the given queries.
func NewNoteService(queries *store.Queries) *NoteService {
	return &NoteService{queries: queries}
}

// Create inserts a new note for the given user and returns the created note.
func (s *NoteService) Create(ctx context.Context, userID, username, title, content string) (*model.Note, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	t, err := s.queries.CreateNote(ctx, store.CreateNoteParams{
		Title:   title,
		Content: content,
		UserID:  uid,
	})
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}

	slog.Info("created note", "id", t.ID.String())
	return &model.Note{
		ID:         t.ID.String(),
		Title:      t.Title,
		Content:    t.Content,
		CreatedAt:  t.CreatedAt,
		ModifiedAt: t.ModifiedAt,
		User:       &model.User{ID: t.UserID.String(), Username: username},
	}, nil
}

// GetByID retrieves a note by ID after verifying the requesting user owns it.
// Returns "note not found" for both missing and unauthorized access to avoid
// leaking note existence information.
func (s *NoteService) GetByID(ctx context.Context, userID, noteID string) (*model.Note, error) {
	uid, err := uuid.Parse(noteID)
	if err != nil {
		return nil, fmt.Errorf("invalid note id: %w", err)
	}

	t, err := s.queries.GetNoteByIDWithUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("note not found")
	}
	if t.User.ID != userID {
		slog.Warn("unauthorized note access", "user", userID, "note", noteID)
		return nil, fmt.Errorf("note not found")
	}

	return t, nil
}

// Update updates a note's title/content after verifying ownership. Returns
// "note not found" for unauthorized access (same info-leak prevention as
// GetByID).
func (s *NoteService) Update(ctx context.Context, userID, noteID string, title, content *string) (*model.Note, error) {
	uid, err := uuid.Parse(noteID)
	if err != nil {
		return nil, fmt.Errorf("invalid note id: %w", err)
	}

	t, err := s.queries.GetNoteByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("note not found")
	}
	if t.UserID.String() != userID {
		slog.Warn("unauthorized note update", "user", userID, "note", noteID)
		return nil, fmt.Errorf("note not found")
	}

	if _, err := s.queries.UpdateNotePartial(ctx, uid, title, content); err != nil {
		return nil, fmt.Errorf("update note: %w", err)
	}

	t2, err := s.queries.GetNoteByIDWithUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("note not found after update")
	}

	slog.Info("updated note", "id", noteID)
	return t2, nil
}

// Delete deletes a note after verifying ownership. Returns "note not found"
// for unauthorized access.
func (s *NoteService) Delete(ctx context.Context, userID, noteID string) error {
	uid, err := uuid.Parse(noteID)
	if err != nil {
		return fmt.Errorf("invalid note id: %w", err)
	}

	t, err := s.queries.GetNoteByID(ctx, uid)
	if err != nil {
		return fmt.Errorf("note not found")
	}
	if t.UserID.String() != userID {
		slog.Warn("unauthorized note delete", "user", userID, "note", noteID)
		return fmt.Errorf("note not found")
	}

	if err := s.queries.DeleteNote(ctx, uid); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}

	slog.Info("deleted note", "id", noteID)
	return nil
}

// List retrieves a paginated, filterable, searchable list of notes owned by
// the given user.
func (s *NoteService) List(ctx context.Context, userID string, input store.QueryInput) (*model.NoteConnection, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}
	return s.queries.FindNotesPaginated(ctx, uid, input)
}
