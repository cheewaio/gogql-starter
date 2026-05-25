package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/cheewaio/gogql-starter/internal/store"
	"github.com/google/uuid"
)

type NoteService struct {
	queries *store.Queries
}

func NewNoteService(queries *store.Queries) *NoteService {
	return &NoteService{queries: queries}
}

func (s *NoteService) Create(ctx context.Context, userID, username, content string) (*model.Note, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	t, err := s.queries.CreateNote(ctx, store.CreateNoteParams{
		Content: content,
		UserID:  uid,
	})
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}

	slog.Info("created note", "id", t.ID.String())
	return &model.Note{
		ID:         t.ID.String(),
		Content:    t.Content,
		CreatedAt:  t.CreatedAt,
		ModifiedAt: t.ModifiedAt,
		User:       &model.User{ID: t.UserID.String(), Username: username},
	}, nil
}

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

func (s *NoteService) Update(ctx context.Context, userID, noteID string, content *string) (*model.Note, error) {
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

	if _, err := s.queries.UpdateNotePartial(ctx, uid, content); err != nil {
		return nil, fmt.Errorf("update note: %w", err)
	}

	t2, err := s.queries.GetNoteByIDWithUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("note not found after update")
	}

	slog.Info("updated note", "id", noteID)
	return t2, nil
}

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

func (s *NoteService) List(ctx context.Context, page, pageSize int32, filters []*model.FilterCriteria, logic model.FilterLogic) (*model.NoteConnection, error) {
	return s.queries.FindNotesPaginated(ctx, page, pageSize, filters, logic)
}
