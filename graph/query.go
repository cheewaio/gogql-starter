package graph

import (
	"fmt"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/cheewaio/gogql-starter/internal/store"
)

func clamp(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func parseQueryInput(input *model.PageInput, filter *model.FilterInput) (store.QueryInput, error) {
	var q store.QueryInput

	switch {
	case input != nil && input.Before != nil:
		sortVals, id, err := store.DecodeCursor(*input.Before)
		if err != nil {
			return q, fmt.Errorf("invalid before cursor: %w", err)
		}
		q.Before = &store.Cursor{SortValues: sortVals, ID: id}

		q.Last = 10
		if input.Last != nil {
			q.Last = clamp(*input.Last, 1, 50)
		} else if input.First != nil {
			return q, fmt.Errorf("last must be used with before, not first")
		}

		if input.After != nil {
			return q, fmt.Errorf("after and before cannot be used together")
		}

	case input != nil && input.After != nil:
		sortVals, id, err := store.DecodeCursor(*input.After)
		if err != nil {
			return q, fmt.Errorf("invalid after cursor: %w", err)
		}
		q.After = &store.Cursor{SortValues: sortVals, ID: id}

		q.First = 10
		if input.First != nil {
			q.First = clamp(*input.First, 1, 50)
		} else if input.Last != nil {
			return q, fmt.Errorf("first must be used with after, not last")
		}

		if input.Before != nil {
			return q, fmt.Errorf("after and before cannot be used together")
		}

	default:
		q.First = 10
		if input != nil && input.First != nil {
			q.First = clamp(*input.First, 1, 50)
		} else if input != nil && input.Last != nil {
			q.Last = clamp(*input.Last, 1, 50)
			q.First = 0
		}
	}

	if input != nil && input.Sort != nil {
		for _, sf := range input.Sort {
			if sf != nil {
				q.Sort = append(q.Sort, store.SortField{
					Field: sf.Field,
					Asc:   sf.Asc,
				})
			}
		}
	}

	q.FilterLogic = model.FilterLogicAnd
	if filter != nil {
		if filter.Filters != nil {
			q.Filters = filter.Filters
		}
		q.FilterLogic = filter.Logic
	}

	return q, nil
}
