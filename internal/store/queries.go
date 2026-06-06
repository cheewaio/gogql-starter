package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cheewaio/gogql-starter/graph/model"
)

var noteFields = map[string]string{
	"id":          "id",
	"title":       "title",
	"content":     "content",
	"created_at":  "created_at",
	"createdAt":   "created_at",
	"modified_at": "modified_at",
	"modifiedAt":  "modified_at",
}

// NormalizeNoteField returns the SQL column name for a public note field.
func NormalizeNoteField(field string) (string, error) {
	column, ok := noteFields[field]
	if !ok {
		return "", fmt.Errorf("unsupported note field: %s", field)
	}
	return column, nil
}

// directionOp returns the SQL comparison operator for the relationship between
// a sort field's declared direction (Asc) and the pagination direction
// (forward = after-cursor / first page). When they match, later rows have
// greater values (>); when they conflict, later rows have smaller values (<).
func directionOp(asc, forward bool) string {
	if asc == forward {
		return ">"
	}
	return "<"
}

// buildCursorWhereClause generates the SQL WHERE clause for keyset pagination
// using the "seek method". For each sort field, it produces an OR branch that
// pins earlier fields to the cursor value and compares the current field with
// the cursor value. The final branch additionally pins the tiebreaker sort
// values and compares the row ID for stable ordering.
//
// Example for two sort fields (a ASC, b ASC) and forward direction:
//
//	(a > $1) OR (a = $1 AND b > $2) OR (a = $1 AND b = $2 AND t.id > $3)
func buildCursorWhereClause(sortFields []SortField, cursor *Cursor, forward bool, argIdx *int, args *[]any, tableAlias string) (string, error) {
	if len(cursor.SortValues) != len(sortFields) {
		return "", fmt.Errorf("cursor sort value count mismatch")
	}
	pairs, err := cursorSortPairs(sortFields, cursor.SortValues)
	if err != nil {
		return "", err
	}

	var orClauses []string

	for i, pair := range pairs {
		var andParts []string

		for _, prev := range pairs[:i] {
			*argIdx++
			*args = append(*args, prev.value)
			andParts = append(andParts, fmt.Sprintf("%s.%s = $%d", tableAlias, prev.field.Field, *argIdx))
		}
		*argIdx++
		*args = append(*args, pair.value)
		op := directionOp(pair.field.Asc, forward)
		andParts = append(andParts, fmt.Sprintf("%s.%s %s $%d", tableAlias, pair.field.Field, op, *argIdx))

		orClauses = append(orClauses, "("+strings.Join(andParts, " AND ")+")")
	}

	var idParts []string
	for _, pair := range pairs {
		*argIdx++
		*args = append(*args, pair.value)
		idParts = append(idParts, fmt.Sprintf("%s.%s = $%d", tableAlias, pair.field.Field, *argIdx))
	}
	*argIdx++
	*args = append(*args, cursor.ID)
	idOp := ">"
	if !forward {
		idOp = "<"
	}
	idParts = append(idParts, fmt.Sprintf("%s.id %s $%d", tableAlias, idOp, *argIdx))
	orClauses = append(orClauses, "("+strings.Join(idParts, " AND ")+")")

	return "(" + strings.Join(orClauses, " OR ") + ")", nil
}

type cursorSortPair struct {
	field SortField
	value any
}

func cursorSortPairs(sortFields []SortField, sortValues []string) ([]cursorSortPair, error) {
	pairs := make([]cursorSortPair, 0, len(sortFields))
	remainingValues := sortValues
	for _, field := range sortFields {
		rawValue := remainingValues[0] //nolint:gosec // length equality is checked by caller before slicing.
		remainingValues = remainingValues[1:]
		value, err := cursorSortValue(field.Field, rawValue)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, cursorSortPair{field: field, value: value})
	}
	return pairs, nil
}

func cursorSortValue(field, value string) (any, error) {
	switch field {
	case "created_at", "modified_at":
		millis, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor timestamp for %s: %w", field, err)
		}
		return time.UnixMilli(millis), nil
	default:
		return value, nil
	}
}

// buildSortOrder generates the SQL ORDER BY clause for keyset pagination.
// When forward is false (Before cursor), the sort direction is inverted so
// that the "rows before the cursor" come first in the result. The row ID is
// appended as a tiebreaker for stable ordering.
func buildSortOrder(sortFields []SortField, forward bool, tableAlias string) string {
	var clauses []string
	for _, sf := range sortFields {
		dir := "ASC"
		if !sf.Asc {
			dir = "DESC"
		}
		if !forward {
			if dir == "ASC" {
				dir = "DESC"
			} else {
				dir = "ASC"
			}
		}
		clauses = append(clauses, fmt.Sprintf("%s.%s %s", tableAlias, sf.Field, dir))
	}
	if forward {
		clauses = append(clauses, fmt.Sprintf("%s.id ASC", tableAlias))
	} else {
		clauses = append(clauses, fmt.Sprintf("%s.id DESC", tableAlias))
	}
	return strings.Join(clauses, ", ")
}

// buildWhereClause converts a single FilterCriteria into a parameterized SQL
// WHERE fragment. The field is qualified with the given table alias. Supported
// operators: eq, neq, contains (ILIKE), gt, gte, lt, lte, isNull, isNotNull.
func buildWhereClause(f *model.FilterCriteria, argIdx *int, args *[]any, tableAlias string) (string, error) {
	col, err := NormalizeNoteField(f.Field)
	if err != nil {
		return "", err
	}

	nextArg := func(value any) int {
		*argIdx++
		*args = append(*args, value)
		return *argIdx
	}

	switch f.Operator {
	case model.FilterOperatorEq:
		idx := nextArg(f.Value)
		return fmt.Sprintf("%s.%s = $%d", tableAlias, col, idx), nil
	case model.FilterOperatorNeq:
		idx := nextArg(f.Value)
		return fmt.Sprintf("%s.%s <> $%d", tableAlias, col, idx), nil
	case model.FilterOperatorContains:
		v := ""
		if f.Value != nil {
			v = *f.Value
		}
		idx := nextArg("%" + v + "%")
		return fmt.Sprintf("%s.%s ILIKE $%d", tableAlias, col, idx), nil
	case model.FilterOperatorGt:
		idx := nextArg(f.Value)
		return fmt.Sprintf("%s.%s > $%d", tableAlias, col, idx), nil
	case model.FilterOperatorGte:
		idx := nextArg(f.Value)
		return fmt.Sprintf("%s.%s >= $%d", tableAlias, col, idx), nil
	case model.FilterOperatorLt:
		idx := nextArg(f.Value)
		return fmt.Sprintf("%s.%s < $%d", tableAlias, col, idx), nil
	case model.FilterOperatorLte:
		idx := nextArg(f.Value)
		return fmt.Sprintf("%s.%s <= $%d", tableAlias, col, idx), nil
	case model.FilterOperatorIsNull:
		return fmt.Sprintf("%s.%s IS NULL", tableAlias, col), nil
	case model.FilterOperatorIsNotNull:
		return fmt.Sprintf("%s.%s IS NOT NULL", tableAlias, col), nil
	default:
		return "", fmt.Errorf("unknown operator: %s", f.Operator)
	}
}

func buildFilterGroupClause(filters []*model.FilterCriteria, logic model.FilterLogic, argIdx *int, args *[]any, tableAlias string) (string, error) {
	var clauses []string
	for _, f := range filters {
		if f == nil {
			continue
		}
		clause, err := buildWhereClause(f, argIdx, args, tableAlias)
		if err != nil {
			return "", err
		}
		if clause != "" {
			clauses = append(clauses, clause)
		}
	}
	if len(clauses) == 0 {
		return "", nil
	}

	joiner := " AND "
	if logic == model.FilterLogicOr {
		joiner = " OR "
	}
	return "(" + strings.Join(clauses, joiner) + ")", nil
}

// buildSearchClause generates an ILIKE WHERE fragment that ORs the search
// query across the given fields. If fields is empty, it falls back to
// defaultFields (entity-specific). Returns an empty string if there is nothing
// to search (nil search, empty query, or no fields).
func buildSearchClause(search *model.SearchInput, argIdx *int, args *[]any, tableAlias string, defaultFields []string) (string, error) {
	if search == nil || search.Query == "" {
		return "", nil
	}

	fields := search.Fields
	if len(fields) == 0 {
		fields = defaultFields
	}

	if len(fields) == 0 {
		return "", nil
	}

	var clauses []string
	pattern := "%" + search.Query + "%"
	for _, field := range fields {
		column, err := NormalizeNoteField(field)
		if err != nil {
			return "", err
		}
		*argIdx++
		*args = append(*args, pattern)
		clauses = append(clauses, fmt.Sprintf("%s.%s ILIKE $%d", tableAlias, column, *argIdx))
	}

	return "(" + strings.Join(clauses, " OR ") + ")", nil
}
