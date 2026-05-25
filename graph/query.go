package graph

import "github.com/cheewaio/gogql-starter/graph/model"

type QueryInput struct {
	Page        int32
	PageSize    int32
	Filters     []*model.FilterCriteria
	FilterLogic model.FilterLogic
}

func parseQueryInput(input *model.PageInput, filter *model.FilterInput) QueryInput {
	q := QueryInput{
		Page:     1,
		PageSize: 20,
		Filters:  nil,
	}
	if input != nil {
		q.Page = input.Page
		q.PageSize = input.PageSize
	}

	q.FilterLogic = model.FilterLogicAnd
	if filter != nil {
		if filter.Filters != nil {
			q.Filters = filter.Filters
		}
		q.FilterLogic = filter.Logic
	}

	return q
}
