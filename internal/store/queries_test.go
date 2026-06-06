package store

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/google/uuid"
)

func TestNormalizeNoteField(t *testing.T) {
	tests := map[string]string{
		"id":          "id",
		"title":       "title",
		"content":     "content",
		"created_at":  "created_at",
		"createdAt":   "created_at",
		"modified_at": "modified_at",
		"modifiedAt":  "modified_at",
	}

	for input, want := range tests {
		got, err := NormalizeNoteField(input)
		if err != nil {
			t.Fatalf("NormalizeNoteField(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeNoteField(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeNoteFieldRejectsInvalidField(t *testing.T) {
	if _, err := NormalizeNoteField("title; DROP TABLE notes"); err == nil {
		t.Fatal("expected invalid field error")
	}
}

func TestDecodePageCursorDirection(t *testing.T) {
	tests := []struct {
		name      string
		cursor    string
		direction CursorDirection
	}{
		{
			name:      "next",
			cursor:    EncodePageCursor([]string{"1780170302035"}, "550e8400-e29b-41d4-a716-446655440000", CursorDirectionNext),
			direction: CursorDirectionNext,
		},
		{
			name:      "previous",
			cursor:    EncodePageCursor([]string{"1780170302035"}, "550e8400-e29b-41d4-a716-446655440000", CursorDirectionPrevious),
			direction: CursorDirectionPrevious,
		},
		{
			name:      "legacy",
			cursor:    legacyCursor([]string{"1780170302035"}, "550e8400-e29b-41d4-a716-446655440000"),
			direction: CursorDirectionNext,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sortValues, id, direction, err := DecodePageCursor(tc.cursor)
			if err != nil {
				t.Fatal(err)
			}
			if direction != tc.direction {
				t.Fatalf("direction = %d, want %d", direction, tc.direction)
			}
			if id != "550e8400-e29b-41d4-a716-446655440000" {
				t.Fatalf("id = %q, want test uuid", id)
			}
			if len(sortValues) != 1 || sortValues[0] != "1780170302035" {
				t.Fatalf("sort values = %+v, want timestamp millis", sortValues)
			}
		})
	}
}

func TestBuildFilterGroupClauseHonorsLogic(t *testing.T) {
	title := "hello"
	content := "world"
	filters := []*model.FilterCriteria{
		{Field: "title", Operator: model.FilterOperatorEq, Value: &title},
		{Field: "content", Operator: model.FilterOperatorContains, Value: &content},
	}

	for _, tc := range []struct {
		name  string
		logic model.FilterLogic
		want  string
	}{
		{name: "and", logic: model.FilterLogicAnd, want: "(t.title = $1 AND t.content ILIKE $2)"},
		{name: "or", logic: model.FilterLogicOr, want: "(t.title = $1 OR t.content ILIKE $2)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				argIdx int
				args   []any
			)
			got, err := buildFilterGroupClause(filters, tc.logic, &argIdx, &args, "t")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("clause = %q, want %q", got, tc.want)
			}
			if argIdx != 2 || len(args) != 2 {
				t.Fatalf("argIdx/args = %d/%d, want 2/2", argIdx, len(args))
			}
		})
	}
}

func TestBuildFilterGroupClauseRejectsInvalidField(t *testing.T) {
	var (
		argIdx int
		args   []any
	)
	_, err := buildFilterGroupClause(
		[]*model.FilterCriteria{{Field: "title; DROP TABLE notes", Operator: model.FilterOperatorEq}},
		model.FilterLogicAnd,
		&argIdx,
		&args,
		"t",
	)
	if err == nil {
		t.Fatal("expected invalid field error")
	}
	if argIdx != 0 || len(args) != 0 {
		t.Fatalf("argIdx/args = %d/%d, want 0/0", argIdx, len(args))
	}
}

func TestBuildWhereClauseDoesNotAllocateArgsForNullOperators(t *testing.T) {
	var (
		argIdx int
		args   []any
	)

	clause, err := buildWhereClause(&model.FilterCriteria{Field: "title", Operator: model.FilterOperatorIsNull}, &argIdx, &args, "t")
	if err != nil {
		t.Fatal(err)
	}
	if clause != "t.title IS NULL" {
		t.Fatalf("clause = %q, want %q", clause, "t.title IS NULL")
	}
	if argIdx != 0 || len(args) != 0 {
		t.Fatalf("argIdx/args = %d/%d, want 0/0", argIdx, len(args))
	}
}

func TestBuildSearchClauseRejectsInvalidField(t *testing.T) {
	var (
		argIdx int
		args   []any
	)
	_, err := buildSearchClause(
		&model.SearchInput{Query: "hello", Fields: []string{"title; DROP TABLE notes"}},
		&argIdx,
		&args,
		"t",
		[]string{"title", "content"},
	)
	if err == nil {
		t.Fatal("expected invalid field error")
	}
}

func TestBuildSortOrder(t *testing.T) {
	tests := []struct {
		name    string
		sort    []SortField
		forward bool
		want    string
	}{
		{
			name:    "ascending forward",
			sort:    []SortField{{Field: "title", Asc: true}},
			forward: true,
			want:    "t.title ASC, t.id ASC",
		},
		{
			name:    "ascending backward",
			sort:    []SortField{{Field: "title", Asc: true}},
			forward: false,
			want:    "t.title DESC, t.id DESC",
		},
		{
			name:    "descending forward",
			sort:    []SortField{{Field: "created_at", Asc: false}},
			forward: true,
			want:    "t.created_at DESC, t.id ASC",
		},
		{
			name:    "descending backward",
			sort:    []SortField{{Field: "created_at", Asc: false}},
			forward: false,
			want:    "t.created_at ASC, t.id DESC",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSortOrder(tc.sort, tc.forward, "t")
			if got != tc.want {
				t.Fatalf("order = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildCursorWhereClauseConvertsTimestampCursorValues(t *testing.T) {
	var (
		argIdx int
		args   []any
	)

	clause, err := buildCursorWhereClause(
		[]SortField{{Field: "created_at", Asc: false}},
		&Cursor{SortValues: []string{"1780170302035"}, ID: "550e8400-e29b-41d4-a716-446655440000"},
		true,
		&argIdx,
		&args,
		"t",
	)
	if err != nil {
		t.Fatal(err)
	}
	wantClause := "((t.created_at < $1) OR (t.created_at = $2 AND t.id > $3))"
	if clause != wantClause {
		t.Fatalf("clause = %q, want %q", clause, wantClause)
	}
	if argIdx != 3 || len(args) != 3 {
		t.Fatalf("argIdx/args = %d/%d, want 3/3", argIdx, len(args))
	}
	for _, idx := range []int{0, 1} {
		got, ok := args[idx].(time.Time)
		if !ok {
			t.Fatalf("arg %d type = %T, want time.Time", idx, args[idx])
		}
		if !got.Equal(time.UnixMilli(1780170302035)) {
			t.Fatalf("arg %d = %s, want %s", idx, got, time.UnixMilli(1780170302035))
		}
	}
}

func TestBuildCursorWhereClauseRejectsMalformedCursorValues(t *testing.T) {
	tests := []struct {
		name   string
		cursor *Cursor
	}{
		{
			name:   "wrong sort value count",
			cursor: &Cursor{SortValues: nil, ID: "550e8400-e29b-41d4-a716-446655440000"},
		},
		{
			name:   "invalid timestamp",
			cursor: &Cursor{SortValues: []string{"not-a-timestamp"}, ID: "550e8400-e29b-41d4-a716-446655440000"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				argIdx int
				args   []any
			)
			_, err := buildCursorWhereClause(
				[]SortField{{Field: "created_at", Asc: false}},
				tc.cursor,
				true,
				&argIdx,
				&args,
				"t",
			)
			if err == nil {
				t.Fatal("expected cursor error")
			}
		})
	}
}

func TestBuildCursorPageInfoReturnsPreviousCursorDirection(t *testing.T) {
	items := []*model.Note{
		{ID: "550e8400-e29b-41d4-a716-446655440000", CreatedAt: time.UnixMilli(3000)},
		{ID: "550e8400-e29b-41d4-a716-446655440001", CreatedAt: time.UnixMilli(2000)},
	}

	got := buildCursorPageInfo(items, []SortField{{Field: "created_at", Asc: false}}, 2, true, true, false, false)
	prev, ok := got.Previous.(*model.CursorPage)
	if !ok {
		t.Fatalf("previous = %T, want CursorPage", got.Previous)
	}

	_, id, direction, err := DecodePageCursor(prev.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if direction != CursorDirectionPrevious {
		t.Fatalf("previous cursor direction = %d, want %d", direction, CursorDirectionPrevious)
	}
	if id != items[0].ID {
		t.Fatalf("previous cursor id = %q, want %q", id, items[0].ID)
	}
}

func legacyCursor(sortValues []string, id string) string {
	var buf bytes.Buffer
	buf.WriteByte(byte(len(sortValues))) //nolint:gosec // test cursor field count is tiny.
	for _, v := range sortValues {
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(v))) //nolint:gosec // test value is tiny.
		buf.WriteString(v)
	}
	uid := uuid.MustParse(id)
	buf.Write(uid[:])
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func TestBackwardOverfetchTrimsBeforeReverse(t *testing.T) {
	items := []*model.Note{
		{ID: "5"},
		{ID: "6"},
		{ID: "7"},
		{ID: "8"},
	}

	items, hasMore := trimCursorItems(items, 3)
	if !hasMore {
		t.Fatal("expected overfetch")
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	got := []string{items[0].ID, items[1].ID, items[2].ID}
	want := []string{"7", "6", "5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %+v, want %+v", got, want)
		}
	}
}

func TestNormalizeSortFieldsRejectsInvalidField(t *testing.T) {
	_, err := normalizeSortFields([]SortField{{Field: "title; DROP TABLE notes", Asc: true}})
	if err == nil {
		t.Fatal("expected invalid field error")
	}
	if !strings.Contains(err.Error(), "unsupported note field") {
		t.Fatalf("unexpected error: %v", err)
	}
}
