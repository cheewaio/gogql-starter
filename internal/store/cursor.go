package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/cheewaio/gogql-starter/graph/model"
)

type SortField struct {
	Field string
	Asc   bool
}

type Cursor struct {
	SortValues []string
	ID         string
}

type QueryInput struct {
	First       int32
	After       *Cursor
	Last        int32
	Before      *Cursor
	Sort        []SortField
	Filters     []*model.FilterCriteria
	FilterLogic model.FilterLogic
}

type cursorPayload struct {
	Vals []string `json:"v"`
	ID   string   `json:"i"`
}

func EncodeCursor(sortValues []string, id string) string {
	data, _ := json.Marshal(cursorPayload{Vals: sortValues, ID: id})
	return base64.RawURLEncoding.EncodeToString(data)
}

func DecodeCursor(cursor string) (sortValues []string, id string, err error) {
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, "", fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var p cursorPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, "", fmt.Errorf("invalid cursor payload: %w", err)
	}
	return p.Vals, p.ID, nil
}
