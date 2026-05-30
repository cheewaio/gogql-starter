package store

import (
	"fmt"
	"strings"

	"github.com/cheewaio/gogql-starter/graph/model"
)

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
func buildCursorWhereClause(sortFields []SortField, cursor *Cursor, forward bool, argIdx *int, args *[]any, tableAlias string) string {
	var orClauses []string

	for i := range sortFields {
		var andParts []string

		for j := 0; j < i; j++ {
			*argIdx++
			*args = append(*args, cursor.SortValues[j])
			andParts = append(andParts, fmt.Sprintf("%s.%s = $%d", tableAlias, sortFields[j].Field, *argIdx))
		}

		*argIdx++
		*args = append(*args, cursor.SortValues[i])
		op := directionOp(sortFields[i].Asc, forward)
		andParts = append(andParts, fmt.Sprintf("%s.%s %s $%d", tableAlias, sortFields[i].Field, op, *argIdx))

		orClauses = append(orClauses, "("+strings.Join(andParts, " AND ")+")")
	}

	var idParts []string
	for j := 0; j < len(sortFields); j++ {
		*argIdx++
		*args = append(*args, cursor.SortValues[j])
		idParts = append(idParts, fmt.Sprintf("%s.%s = $%d", tableAlias, sortFields[j].Field, *argIdx))
	}
	*argIdx++
	*args = append(*args, cursor.ID)
	idOp := ">"
	if !forward {
		idOp = "<"
	}
	idParts = append(idParts, fmt.Sprintf("%s.id %s $%d", tableAlias, idOp, *argIdx))
	orClauses = append(orClauses, "("+strings.Join(idParts, " AND ")+")")

	return "(" + strings.Join(orClauses, " OR ") + ")"
}

// buildSortOrder generates the SQL ORDER BY clause for keyset pagination.
// When forward is false (Before cursor), the sort direction is inverted so
// that the "rows before the cursor" come first in the result. The row ID is
// appended as a tiebreaker for stable ordering.
func buildSortOrder(sortFields []SortField, forward bool, tableAlias string) string {
	var clauses []string
	for _, sf := range sortFields {
		dir := "ASC"
		if forward == sf.Asc {
			dir = "DESC"
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
	*argIdx++
	col := f.Field

	switch f.Operator {
	case model.FilterOperatorEq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s.%s = $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorNeq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s.%s <> $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorContains:
		v := ""
		if f.Value != nil {
			v = *f.Value
		}
		*args = append(*args, "%"+v+"%")
		return fmt.Sprintf("%s.%s ILIKE $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorGt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s.%s > $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorGte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s.%s >= $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorLt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s.%s < $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorLte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s.%s <= $%d", tableAlias, col, *argIdx), nil
	case model.FilterOperatorIsNull:
		return fmt.Sprintf("%s.%s IS NULL", tableAlias, col), nil
	case model.FilterOperatorIsNotNull:
		return fmt.Sprintf("%s.%s IS NOT NULL", tableAlias, col), nil
	default:
		return "", fmt.Errorf("unknown operator: %s", f.Operator)
	}
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
		*argIdx++
		*args = append(*args, pattern)
		clauses = append(clauses, fmt.Sprintf("%s.%s ILIKE $%d", tableAlias, field, *argIdx))
	}

	return "(" + strings.Join(clauses, " OR ") + ")", nil
}
