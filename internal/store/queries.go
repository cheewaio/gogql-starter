package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/google/uuid"
)

// GetNoteByIDWithUser fetches a single note by ID, joining the user info.
func (q *Queries) GetNoteByIDWithUser(ctx context.Context, id uuid.UUID) (*model.Note, error) {
	query := `
		SELECT t.id, t.content, t.created_at, t.modified_at, t.user_id, u.username
		FROM notes t
		JOIN users u ON u.id = t.user_id
		WHERE t.id = $1
	`
	var (
		noteID     uuid.UUID
		createdAt  time.Time
		modifiedAt time.Time
		userID     uuid.UUID
		username   string
		note       model.Note
	)
	err := q.db.QueryRowContext(ctx, query, id).Scan(&noteID, &note.Content, &createdAt, &modifiedAt, &userID, &username)
	if err != nil {
		return nil, fmt.Errorf("get note: %w", err)
	}
	note.ID = noteID.String()
	note.CreatedAt = createdAt
	note.ModifiedAt = modifiedAt
	note.User = &model.User{ID: userID.String(), Username: username}
	return &note, nil
}

// UpdateNotePartial updates the content field of a note, bumping modified_at.
func (q *Queries) UpdateNotePartial(ctx context.Context, id uuid.UUID, content *string) (Note, error) {
	var (
		sets []string
		args []any
		idx  int
	)
	if content != nil {
		idx++
		sets = append(sets, fmt.Sprintf("content = $%d", idx))
		args = append(args, *content)
	}
	if len(sets) == 0 {
		return q.GetNoteByID(ctx, id)
	}
	sets = append(sets, "modified_at = NOW()")
	idx++
	query := fmt.Sprintf("UPDATE notes SET %s WHERE id = $%d RETURNING *", strings.Join(sets, ", "), idx)
	args = append(args, id)

	row := q.db.QueryRowContext(ctx, query, args...)
	var i Note
	err := row.Scan(
		&i.ID,
		&i.Content,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.UserID,
	)
	return i, err
}

func (q *Queries) FindNotesPaginated(
	ctx context.Context,
	page, pageSize int32,
	filters []*model.FilterCriteria,
	logic model.FilterLogic,
) (*model.NoteConnection, error) {
	var (
		whereClauses []string
		args         []any
		argIdx       int
	)

	for _, f := range filters {
		if f == nil {
			continue
		}
		clause, err := buildWhereClause(f, &argIdx, &args)
		if err != nil {
			return nil, fmt.Errorf("build filter: %w", err)
		}
		if clause != "" {
			whereClauses = append(whereClauses, clause)
		}
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		op := "AND"
		if logic == model.FilterLogicOr {
			op = "OR"
		}
		whereSQL = "WHERE " + strings.Join(whereClauses, " "+op+" ")
	}

	var total int32
	countQuery := "SELECT COUNT(*) FROM notes " + whereSQL
	if err := q.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count notes: %w", err)
	}

	offset := (page - 1) * pageSize
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.content, t.created_at, t.modified_at, t.user_id, u.username
		FROM notes t
		JOIN users u ON u.id = t.user_id
		%s
		ORDER BY t.id
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx+1, argIdx+2)

	args = append(args, pageSize, offset)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []*model.Note
	for rows.Next() {
		var (
			noteID     uuid.UUID
			createdAt  time.Time
			modifiedAt time.Time
			userID     uuid.UUID
			username   string
			note       model.Note
		)
		if err := rows.Scan(&noteID, &note.Content, &createdAt, &modifiedAt, &userID, &username); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		note.ID = noteID.String()
		note.CreatedAt = createdAt
		note.ModifiedAt = modifiedAt
		note.User = &model.User{ID: userID.String(), Username: username}
		items = append(items, &note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if items == nil {
		items = []*model.Note{}
	}

	return &model.NoteConnection{
		Items: items,
		PageInfo: &model.PageInfo{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

func buildWhereClause(f *model.FilterCriteria, argIdx *int, args *[]any) (string, error) {
	*argIdx++
	col := f.Field

	switch f.Operator {
	case model.FilterOperatorEq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("t.%s = $%d", col, *argIdx), nil
	case model.FilterOperatorNeq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("t.%s <> $%d", col, *argIdx), nil
	case model.FilterOperatorContains:
		v := ""
		if f.Value != nil {
			v = *f.Value
		}
		*args = append(*args, "%"+v+"%")
		return fmt.Sprintf("t.%s ILIKE $%d", col, *argIdx), nil
	case model.FilterOperatorGt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("t.%s > $%d", col, *argIdx), nil
	case model.FilterOperatorGte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("t.%s >= $%d", col, *argIdx), nil
	case model.FilterOperatorLt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("t.%s < $%d", col, *argIdx), nil
	case model.FilterOperatorLte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("t.%s <= $%d", col, *argIdx), nil
	case model.FilterOperatorIsNull:
		return fmt.Sprintf("t.%s IS NULL", col), nil
	case model.FilterOperatorIsNotNull:
		return fmt.Sprintf("t.%s IS NOT NULL", col), nil
	default:
		return "", fmt.Errorf("unknown operator: %s", f.Operator)
	}
}
