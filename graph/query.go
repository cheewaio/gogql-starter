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

	mode := model.PaginationModeCursor
	if input != nil && input.Mode != nil {
		mode = *input.Mode
	}

	switch mode {
	case model.PaginationModeCursor:
		q.First = pageSize
		if input != nil && input.Cursor != nil && *input.Cursor != "" {
			sortVals, id, err := store.DecodeCursor(*input.Cursor)
			if err != nil {
				return q, fmt.Errorf("invalid cursor: %w", err)
			}
			q.After = &store.Cursor{SortValues: sortVals, ID: id}
		}

	case model.PaginationModeOffset:
		pageNumber := int32(0)
		if input != nil && input.PageNumber != nil {
			pageNumber = *input.PageNumber
		}
		q.PageNumber = &pageNumber
	}

	if input != nil && input.Sort != nil {
		for _, sf := range input.Sort {
			if sf != nil {
				q.Sort = append(q.Sort, store.SortField{
					Field: sf.Field,
					Asc:   sf.Order != nil && *sf.Order == model.SortOrderAsc,
				})
			}
		}
	}

	if input != nil && input.Filter != nil {
		if input.Filter.Filters != nil {
			q.Filters = input.Filter.Filters
		}
		q.FilterLogic = input.Filter.Logic
	}

	if input != nil && input.Search != nil {
		q.Search = input.Search
	}

	return q, nil
}
