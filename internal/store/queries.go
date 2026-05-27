package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/google/uuid"
)

func (q *Queries) GetNoteByIDWithUser(ctx context.Context, id uuid.UUID) (*model.Note, error) {
	query := `
		SELECT t.id, t.title, t.content, t.created_at, t.modified_at, t.user_id, u.username
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
	err := q.db.QueryRowContext(ctx, query, id).Scan(&noteID, &note.Title, &note.Content, &createdAt, &modifiedAt, &userID, &username)
	if err != nil {
		return nil, fmt.Errorf("get note: %w", err)
	}
	note.ID = noteID.String()
	note.CreatedAt = createdAt
	note.ModifiedAt = modifiedAt
	note.User = &model.User{ID: userID.String(), Username: username}
	return &note, nil
}

func (q *Queries) UpdateNotePartial(ctx context.Context, id uuid.UUID, title, content *string) (Note, error) {
	var (
		sets []string
		args []any
		idx  int
	)
	if title != nil {
		idx++
		sets = append(sets, fmt.Sprintf("title = $%d", idx))
		args = append(args, *title)
	}
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
		&i.Title,
		&i.Content,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.UserID,
	)
	return i, err
}

func directionOp(asc, forward bool) string {
	if asc == forward {
		return ">"
	}
	return "<"
}

func buildCursorWhereClause(sortFields []SortField, cursor *Cursor, forward bool, argIdx *int, args *[]any) string {
	var orClauses []string

	for i := range sortFields {
		var andParts []string

		for j := 0; j < i; j++ {
			*argIdx++
			*args = append(*args, cursor.SortValues[j])
			andParts = append(andParts, fmt.Sprintf("t.%s = $%d", sortFields[j].Field, *argIdx))
		}

		*argIdx++
		*args = append(*args, cursor.SortValues[i])
		op := directionOp(sortFields[i].Asc, forward)
		andParts = append(andParts, fmt.Sprintf("t.%s %s $%d", sortFields[i].Field, op, *argIdx))

		orClauses = append(orClauses, "("+strings.Join(andParts, " AND ")+")")
	}

	var idParts []string
	for j := 0; j < len(sortFields); j++ {
		*argIdx++
		*args = append(*args, cursor.SortValues[j])
		idParts = append(idParts, fmt.Sprintf("t.%s = $%d", sortFields[j].Field, *argIdx))
	}
	*argIdx++
	*args = append(*args, cursor.ID)
	idOp := ">"
	if !forward {
		idOp = "<"
	}
	idParts = append(idParts, fmt.Sprintf("t.id %s $%d", idOp, *argIdx))
	orClauses = append(orClauses, "("+strings.Join(idParts, " AND ")+")")

	return "(" + strings.Join(orClauses, " OR ") + ")"
}

func buildSortOrder(sortFields []SortField, forward bool) string {
	var clauses []string
	for _, sf := range sortFields {
		dir := "ASC"
		if forward == sf.Asc {
			dir = "DESC"
		}
		clauses = append(clauses, fmt.Sprintf("t.%s %s", sf.Field, dir))
	}
	if forward {
		clauses = append(clauses, "t.id ASC")
	} else {
		clauses = append(clauses, "t.id DESC")
	}
	return strings.Join(clauses, ", ")
}

func sortValueFromNote(note *model.Note, field string) string {
	switch field {
	case "created_at":
		return note.CreatedAt.Format(time.RFC3339Nano)
	case "modified_at":
		return note.ModifiedAt.Format(time.RFC3339Nano)
	case "title":
		return note.Title
	case "id":
		return note.ID
	default:
		return ""
	}
}

func sortValuesFromNote(note *model.Note, sortFields []SortField) []string {
	vals := make([]string, len(sortFields))
	for i, sf := range sortFields {
		vals[i] = sortValueFromNote(note, sf.Field)
	}
	return vals
}

func (q *Queries) FindNotesPaginated(
	ctx context.Context,
	userID uuid.UUID,
	input QueryInput,
) (*model.NoteConnection, error) {
	var (
		whereClauses []string
		args         []any
		argIdx       int
	)

	argIdx++
	whereClauses = append(whereClauses, fmt.Sprintf("t.user_id = $%d", argIdx))
	args = append(args, userID)

	forward := input.Before == nil
	useFirst := input.Before == nil

	var cursor *Cursor
	if input.After != nil {
		cursor = input.After
	} else if input.Before != nil {
		cursor = input.Before
	}

	limit := input.First
	if !useFirst {
		limit = input.Last
	}

	sortFields := input.Sort
	if len(sortFields) == 0 {
		sortFields = []SortField{{Field: "created_at", Asc: false}}
	}

	if cursor != nil {
		clause := buildCursorWhereClause(sortFields, cursor, forward, &argIdx, &args)
		whereClauses = append(whereClauses, clause)
	}

	for _, f := range input.Filters {
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

	whereSQL := strings.Join(whereClauses, " AND ")

	orderSQL := buildSortOrder(sortFields, forward)

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.content, t.created_at, t.modified_at, t.user_id, u.username
		FROM notes t
		JOIN users u ON u.id = t.user_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d
	`, whereSQL, orderSQL, argIdx+1)

	args = append(args, limit+1)

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
			uid        uuid.UUID
			username   string
			note       model.Note
		)
		if err := rows.Scan(&noteID, &note.Title, &note.Content, &createdAt, &modifiedAt, &uid, &username); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		note.ID = noteID.String()
		note.CreatedAt = createdAt
		note.ModifiedAt = modifiedAt
		note.User = &model.User{ID: uid.String(), Username: username}
		items = append(items, &note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if !useFirst {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}

	hasNextPage := false
	hasPreviousPage := false
	if len(items) > int(limit) {
		if forward {
			hasNextPage = true
		} else {
			hasPreviousPage = true
		}
		items = items[:limit]
	}

	if hasPreviousPage || (forward && input.After != nil) {
		hasPreviousPage = true
	}
	if hasNextPage || (!forward && input.Before != nil) {
		hasNextPage = true
	}

	if items == nil {
		items = []*model.Note{}
	}

	var startCursor, endCursor *string
	if len(items) > 0 {
		sc := EncodeCursor(sortValuesFromNote(items[0], sortFields), items[0].ID)
		startCursor = &sc
		ec := EncodeCursor(sortValuesFromNote(items[len(items)-1], sortFields), items[len(items)-1].ID)
		endCursor = &ec
	}

	return &model.NoteConnection{
		Items: items,
		PageInfo: &model.PageInfo{
			StartCursor:     startCursor,
			EndCursor:       endCursor,
			HasNextPage:     hasNextPage,
			HasPreviousPage: hasPreviousPage,
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
