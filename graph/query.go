package graph

import (
	"fmt"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/cheewaio/gogql-starter/internal/store"
)

// clamp restricts v to the inclusive range [min, max].
func clamp(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// parseQueryInput converts a GraphQL PaginationInput into the internal
// store.QueryInput, decoding cursors and normalizing defaults. It selects
// cursor-based or offset-based pagination based on the Mode field.
func parseQueryInput(input *model.PaginationInput) (store.QueryInput, error) {
	var q store.QueryInput

	pageSize := int32(20)
	if input != nil && input.PageSize != nil {
		pageSize = clamp(*input.PageSize, 1, 100)
	}
	q.PageSize = pageSize

	if err := applyPagination(&q, input, pageSize); err != nil {
		return q, err
	}
	if err := applySort(&q, input); err != nil {
		return q, err
	}
	if err := applyFilter(&q, input); err != nil {
		return q, err
	}
	if err := applySearch(&q, input); err != nil {
		return q, err
	}

	return q, nil
}

func applyPagination(q *store.QueryInput, input *model.PaginationInput, pageSize int32) error {
	mode := model.PaginationModeCursor
	if input != nil && input.Mode != nil {
		mode = *input.Mode
	}

	switch mode {
	case model.PaginationModeCursor:
		q.First = pageSize
		if input != nil && input.Cursor != nil && *input.Cursor != "" {
			cursor, direction, err := decodeCursor(*input.Cursor)
			if err != nil {
				return err
			}
			switch direction {
			case store.CursorDirectionNext:
				q.After = cursor
			case store.CursorDirectionPrevious:
				q.First = 0
				q.Last = pageSize
				q.Before = cursor
			}
		}

	case model.PaginationModeOffset:
		pageNumber := int32(0)
		if input != nil && input.PageNumber != nil {
			pageNumber = *input.PageNumber
			if pageNumber < 0 {
				pageNumber = 0
			}
		}
		q.PageNumber = &pageNumber
	}
	return nil
}

func applySort(q *store.QueryInput, input *model.PaginationInput) error {
	if input != nil && input.Sort != nil {
		for _, sf := range input.Sort {
			if sf != nil {
				field, err := store.NormalizeNoteField(sf.Field)
				if err != nil {
					return err
				}
				q.Sort = append(q.Sort, store.SortField{
					Field: field,
					Asc:   sf.Order != nil && *sf.Order == model.SortOrderAsc,
				})
			}
		}
	}
	return nil
}

func applyFilter(q *store.QueryInput, input *model.PaginationInput) error {
	if input != nil && input.Filter != nil {
		for _, f := range input.Filter.Filters {
			if f == nil {
				continue
			}
			field, err := store.NormalizeNoteField(f.Field)
			if err != nil {
				return err
			}
			filter := *f
			filter.Field = field
			q.Filters = append(q.Filters, &filter)
		}
		q.FilterLogic = input.Filter.Logic
	}
	return nil
}

func applySearch(q *store.QueryInput, input *model.PaginationInput) error {
	if input != nil && input.Search != nil {
		search := *input.Search
		if len(search.Fields) > 0 {
			search.Fields = make([]string, 0, len(input.Search.Fields))
			for _, field := range input.Search.Fields {
				normalized, err := store.NormalizeNoteField(field)
				if err != nil {
					return err
				}
				search.Fields = append(search.Fields, normalized)
			}
		}
		q.Search = &search
	}
	return nil
}

func decodeCursor(cursorInput string) (*store.Cursor, store.CursorDirection, error) {
	sortVals, id, direction, err := store.DecodePageCursor(cursorInput)
	if err != nil {
		return nil, store.CursorDirectionNext, fmt.Errorf("invalid cursor: %w", err)
	}
	return &store.Cursor{SortValues: sortVals, ID: id}, direction, nil
}
