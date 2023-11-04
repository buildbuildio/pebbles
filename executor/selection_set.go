package executor

import (
	"github.com/buildbuildio/pebbles/common"

	"github.com/vektah/gqlparser/v2/ast"
)

func getFieldDisplayName(field *ast.Field) string {
	if field.Alias != "" {
		return field.Alias
	}
	return field.Name
}

func FindSelection(matchString string, selectionSet ast.SelectionSet) *ast.Field {
	for _, s := range common.SelectionSetToFields(selectionSet, nil) {
		if getFieldDisplayName(s) == matchString {
			return s
		}

		if len(s.SelectionSet) > 0 {
			if f := FindSelection(matchString, s.SelectionSet); f != nil {
				return f
			}
		}
	}

	return nil
}
