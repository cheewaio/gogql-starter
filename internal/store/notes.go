package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/google/uuid"
)

// GetNoteByIDWithUser retrieves a single note by ID with the associated user's
// username in a single JOIN query. This is the primary read path for note
// detail views; prefer GetNoteByID when the user object is not needed.
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

// UpdateNotePartial performs a partial update of a note's title and/or content.
// Only the non-nil fields are included in the SQL SET clause; modified_at is
// always bumped to NOW(). If both title and content are nil, it fetches and
// returns the existing note unchanged.
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

// FindNotesPaginated dispatches to offset-based or cursor-based pagination
// depending on whether PageNumber is set. When PageNumber is nil, cursor-based
// pagination is used (the default).
func (q *Queries) FindNotesPaginated(
	ctx context.Context,
	userID uuid.UUID,
	input QueryInput,
) (*model.NoteConnection, error) {
	if input.PageNumber != nil {
		return q.findNotesOffset(ctx, userID, input)
	}
	return q.findNotesCursor(ctx, userID, input)
}

func (q *Queries) findNotesOffset(
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

	sortFields := input.Sort
	if len(sortFields) == 0 {
		sortFields = []SortField{{Field: "created_at", Asc: false}}
	}

	for _, f := range input.Filters {
		if f == nil {
			continue
		}
		clause, err := buildWhereClause(f, &argIdx, &args, "t")
		if err != nil {
			return nil, fmt.Errorf("build filter: %w", err)
		}
		if clause != "" {
			whereClauses = append(whereClauses, clause)
		}
	}

	if input.Search != nil {
		clause, err := buildSearchClause(input.Search, &argIdx, &args, "t", []string{"title", "content"})
		if err != nil {
			return nil, fmt.Errorf("build search: %w", err)
		}
		if clause != "" {
			whereClauses = append(whereClauses, clause)
		}
	}

	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	pageNumber := int32(0)
	if input.PageNumber != nil {
		pageNumber = *input.PageNumber
	}
	offset := pageNumber * pageSize

	whereSQL := strings.Join(whereClauses, " AND ")

	var orderClauses []string
	for _, sf := range sortFields {
		dir := "ASC"
		if !sf.Asc {
			dir = "DESC"
		}
		orderClauses = append(orderClauses, fmt.Sprintf("t.%s %s", sf.Field, dir))
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.content, t.created_at, t.modified_at, t.user_id, u.username,
		       COUNT(*) OVER() as total_count
		FROM notes t
		JOIN users u ON u.id = t.user_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, strings.Join(orderClauses, ", "), argIdx+1, argIdx+2)

	args = append(args, pageSize, offset)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, total, err := scanNoteRows(rows)
	if err != nil {
		return nil, err
	}

	if items == nil {
		items = []*model.Note{}
	}

	hasNext := total > (pageNumber+1)*pageSize
	hasPrev := pageNumber > 0

	var next model.PaginationPage
	if hasNext {
		n := pageNumber + 1
		next = &model.OffsetPage{PageSize: pageSize, PageNumber: n}
	}
	var prev model.PaginationPage
	if hasPrev {
		p := pageNumber - 1
		prev = &model.OffsetPage{PageSize: pageSize, PageNumber: p}
	}

	return &model.NoteConnection{
		Items: items,
		Pagination: &model.PaginationMetadata{
			Next:     next,
			Previous: prev,
			Total:    total,
		},
	}, nil
}

func (q *Queries) findNotesCursor(
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

	// Determine direction: when Before is set, paginate backward (DESC order),
	// otherwise forward (ASC order). forward and useFirst are equivalent in
	// this codebase — both are true when Before is nil.
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
		clause := buildCursorWhereClause(sortFields, cursor, forward, &argIdx, &args, "t")
		whereClauses = append(whereClauses, clause)
	}

	for _, f := range input.Filters {
		if f == nil {
			continue
		}
		clause, err := buildWhereClause(f, &argIdx, &args, "t")
		if err != nil {
			return nil, fmt.Errorf("build filter: %w", err)
		}
		if clause != "" {
			whereClauses = append(whereClauses, clause)
		}
	}

	if input.Search != nil {
		clause, err := buildSearchClause(input.Search, &argIdx, &args, "t", []string{"title", "content"})
		if err != nil {
			return nil, fmt.Errorf("build search: %w", err)
		}
		if clause != "" {
			whereClauses = append(whereClauses, clause)
		}
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	orderSQL := buildSortOrder(sortFields, forward, "t")

	query := fmt.Sprintf(`
		SELECT t.id, t.title, t.content, t.created_at, t.modified_at, t.user_id, u.username,
		       COUNT(*) OVER() as total_count
		FROM notes t
		JOIN users u ON u.id = t.user_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d
	`, whereSQL, orderSQL, argIdx+1)

	// Fetch one extra row beyond the requested page to detect whether there
	// is a next/previous page (the "over-fetch" pattern for keyset pagination).
	args = append(args, limit+1)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items, total, err := scanNoteRows(rows)
	if err != nil {
		return nil, err
	}

	// When paginating backward (Before cursor), results come in descending sort
	// order. Reverse them so the client receives items in ascending display order.
	if !forward {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}

	cursorPageInfo := buildCursorPageInfo(items, sortFields, limit, forward, input.After != nil, input.Before != nil)

	return &model.NoteConnection{
		Items: cursorPageInfo.Items,
		Pagination: &model.PaginationMetadata{
			Next:     cursorPageInfo.Next,
			Previous: cursorPageInfo.Previous,
			Total:    total,
		},
	}, nil
}

// scanNoteRows iterates over sql.Rows and scans each row into a model.Note
// with its associated user. It uses COUNT(*) OVER() from the query to
// populate the total count, avoiding a separate COUNT query.
func scanNoteRows(rows *sql.Rows) ([]*model.Note, int32, error) {
	var items []*model.Note
	var total int32
	for rows.Next() {
		var (
			noteID     uuid.UUID
			createdAt  time.Time
			modifiedAt time.Time
			uid        uuid.UUID
			username   string
			totalCount int64
		)
		note := model.Note{}
		if err := rows.Scan(&noteID, &note.Title, &note.Content, &createdAt, &modifiedAt, &uid, &username, &totalCount); err != nil {
			return nil, 0, fmt.Errorf("scan note: %w", err)
		}
		note.ID = noteID.String()
		note.CreatedAt = createdAt
		note.ModifiedAt = modifiedAt
		note.User = &model.User{ID: uid.String(), Username: username}
		items = append(items, &note)
		total = int32(totalCount) //nolint:gosec // total notes won't exceed 2^31
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}
	return items, total, nil
}

// sortValueFromNote extracts a string-encoded sort value from a note for the
// given field. Timestamps are converted to Unix millis for compact cursor
// encoding; string fields are used as-is.
func sortValueFromNote(note *model.Note, field string) string {
	switch field {
	case "created_at":
		return strconv.FormatInt(note.CreatedAt.UnixMilli(), 10)
	case "modified_at":
		return strconv.FormatInt(note.ModifiedAt.UnixMilli(), 10)
	case "title":
		return note.Title
	case "id":
		return note.ID
	default:
		return ""
	}
}

// sortValuesFromNote extracts sort values for all sort fields from a note.
func sortValuesFromNote(note *model.Note, sortFields []SortField) []string {
	vals := make([]string, len(sortFields))
	for i, sf := range sortFields {
		vals[i] = sortValueFromNote(note, sf.Field)
	}
	return vals
}

type cursorPageInfo struct {
	Items    []*model.Note
	Next     model.PaginationPage
	Previous model.PaginationPage
}

// buildCursorPageInfo computes the next/previous cursors and pagination
// metadata from a page of results. It handles the "over-fetch" offset logic:
// if more items were returned than the page limit, the extra row indicates
// there is another page in the forward/backward direction.
//
// hasAfter/hasBefore track whether the client explicitly provided an
// after/before cursor, which determines whether a previous/next page exists
// on the other side.
func buildCursorPageInfo(items []*model.Note, sortFields []SortField, limit int32, forward, hasAfter, hasBefore bool) cursorPageInfo {
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

	if hasPreviousPage || (forward && hasAfter) {
		hasPreviousPage = true
	}
	if hasNextPage || (!forward && hasBefore) {
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

	var next, prev model.PaginationPage
	if hasNextPage && endCursor != nil {
		next = &model.CursorPage{PageSize: limit, Cursor: *endCursor}
	}
	if hasPreviousPage && startCursor != nil {
		prev = &model.CursorPage{PageSize: limit, Cursor: *startCursor}
	}

	return cursorPageInfo{Items: items, Next: next, Previous: prev}
}
