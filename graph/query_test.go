package graph

import (
	"testing"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/cheewaio/gogql-starter/internal/store"
)

func TestParseQueryInputClampsNegativePageNumber(t *testing.T) {
	mode := model.PaginationModeOffset
	pageNumber := int32(-3)

	got, err := parseQueryInput(&model.PaginationInput{
		Mode:       &mode,
		PageNumber: &pageNumber,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.PageNumber == nil {
		t.Fatal("expected page number")
	}
	if *got.PageNumber != 0 {
		t.Fatalf("page number = %d, want 0", *got.PageNumber)
	}
}

func TestParseQueryInputUsesBeforeCursor(t *testing.T) {
	cursor := store.EncodePageCursor([]string{"1780170302035"}, "550e8400-e29b-41d4-a716-446655440000", store.CursorDirectionPrevious)

	got, err := parseQueryInput(&model.PaginationInput{Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if got.Before == nil {
		t.Fatal("expected before cursor")
	}
	if got.After != nil {
		t.Fatal("did not expect after cursor")
	}
	if got.First != 0 || got.Last != 20 {
		t.Fatalf("first/last = %d/%d, want 0/20", got.First, got.Last)
	}
}

func TestParseQueryInputUsesAfterCursor(t *testing.T) {
	cursor := store.EncodeCursor([]string{"1780170302035"}, "550e8400-e29b-41d4-a716-446655440000")

	got, err := parseQueryInput(&model.PaginationInput{Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if got.After == nil {
		t.Fatal("expected after cursor")
	}
	if got.Before != nil {
		t.Fatal("did not expect before cursor")
	}
	if got.First != 20 || got.Last != 0 {
		t.Fatalf("first/last = %d/%d, want 20/0", got.First, got.Last)
	}
}

func TestParseQueryInputNormalizesDynamicFields(t *testing.T) {
	sortOrder := model.SortOrderAsc
	value := "hello"

	got, err := parseQueryInput(&model.PaginationInput{
		Sort: []*model.SortField{
			{Field: "createdAt", Order: &sortOrder},
			{Field: "modifiedAt", Order: &sortOrder},
		},
		Filter: &model.FilterInput{
			Filters: []*model.FilterCriteria{
				{Field: "createdAt", Operator: model.FilterOperatorEq, Value: &value},
			},
		},
		Search: &model.SearchInput{
			Query:  "hello",
			Fields: []string{"modifiedAt", "title"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.Sort[0].Field != "created_at" || got.Sort[1].Field != "modified_at" {
		t.Fatalf("sort fields = %+v, want normalized timestamp columns", got.Sort)
	}
	if got.Filters[0].Field != "created_at" {
		t.Fatalf("filter field = %q, want created_at", got.Filters[0].Field)
	}
	if got.Search.Fields[0] != "modified_at" || got.Search.Fields[1] != "title" {
		t.Fatalf("search fields = %+v, want normalized fields", got.Search.Fields)
	}
}

func TestParseQueryInputRejectsInvalidDynamicFields(t *testing.T) {
	sortOrder := model.SortOrderAsc
	value := "hello"

	tests := []struct {
		name  string
		input *model.PaginationInput
	}{
		{
			name: "sort",
			input: &model.PaginationInput{Sort: []*model.SortField{
				{Field: "title; DROP TABLE notes", Order: &sortOrder},
			}},
		},
		{
			name: "filter",
			input: &model.PaginationInput{Filter: &model.FilterInput{Filters: []*model.FilterCriteria{
				{Field: "title; DROP TABLE notes", Operator: model.FilterOperatorEq, Value: &value},
			}}},
		},
		{
			name: "search",
			input: &model.PaginationInput{Search: &model.SearchInput{
				Query:  "hello",
				Fields: []string{"title; DROP TABLE notes"},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseQueryInput(tc.input); err == nil {
				t.Fatal("expected invalid field error")
			}
		})
	}
}
