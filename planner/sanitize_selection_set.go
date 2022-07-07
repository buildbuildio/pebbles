package planner

import (
	"github.com/buildbuildio/pebbles/common"
	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/ast"
)

func sanitizeSelectionSet(ctx *PlanningContext, selectionSet ast.SelectionSet, insertionPoint []string) (ast.SelectionSet, ScrubFields) {
	scrubFields := make(ScrubFields)
	var result ast.SelectionSet
	for _, s := range selectionSet {
		switch s := s.(type) {
		case *ast.Field:
			if len(s.SelectionSet) != 0 {
				childSelectionSet, sf := sanitizeSelectionSet(ctx, s.SelectionSet, append(insertionPoint, s.Alias))
				scrubFields.Merge(sf)

				var addedFields []string
				childSelectionSet, addedFields = addScrubFieldsToSelectionSet(ctx, childSelectionSet, s.Definition.Type.Name())
				scrubFields = setMissingScrubFieldsForFieldSelectionSet(ctx, insertionPoint, s, scrubFields, addedFields)

				s.SelectionSet = childSelectionSet
			}
			result = addSelectionSetToSanitizedResult(result, s)
		case *ast.FragmentSpread:
			inlineFragment := &ast.InlineFragment{
				TypeCondition:    s.Definition.TypeCondition,
				Directives:       s.Directives,
				SelectionSet:     s.Definition.SelectionSet,
				ObjectDefinition: s.ObjectDefinition,
				Position:         s.Position,
			}
			selSet, sf := sanitizeSelectionSet(ctx, ast.SelectionSet{inlineFragment}, insertionPoint)
			scrubFields.Merge(sf)
			result = addSelectionSetToSanitizedResult(result, selSet...)
		case *ast.InlineFragment:
			childSelectionSet, sf := sanitizeSelectionSet(ctx, s.SelectionSet, insertionPoint)
			scrubFields.Merge(sf)

			var addedFields []string
			childSelectionSet, addedFields = addScrubFieldsToSelectionSet(ctx, childSelectionSet, s.TypeCondition)
			for _, f := range addedFields {
				scrubFields.Set(insertionPoint, s.TypeCondition, f)
			}

			switch s.ObjectDefinition.Kind {
			case ast.Interface:
				childSelectionSet = sanitizeInterfaceInlineFragment(ctx, childSelectionSet, s)
				result = addSelectionSetToSanitizedResult(result, childSelectionSet...)
			case ast.Union:
				childSelectionSet = sanitizeUnionInlineFragment(ctx, childSelectionSet, s)
				result = addSelectionSetToSanitizedResult(result, childSelectionSet...)
			default:
				result = addSelectionSetToSanitizedResult(result, childSelectionSet...)
			}

		}
	}

	return result, scrubFields
}

func sanitizeUnionInlineFragment(ctx *PlanningContext, selectionSet ast.SelectionSet, selection *ast.InlineFragment) ast.SelectionSet {
	selection.SelectionSet = nil
	for _, sel := range selectionSet {
		// when getting the same definition, then unfold it
		if s, ok := sel.(*ast.InlineFragment); ok && selection.ObjectDefinition.Name == s.ObjectDefinition.Name && selection.TypeCondition == s.TypeCondition {
			selection.SelectionSet = addSelectionSetToSanitizedResult(selection.SelectionSet, s.SelectionSet...)
		} else {
			selection.SelectionSet = addSelectionSetToSanitizedResult(selection.SelectionSet, sel)
		}
	}
	// same object, unfold
	if selection.TypeCondition == selection.ObjectDefinition.Name {
		return selection.SelectionSet
	}

	return ast.SelectionSet{selection}
}

func sanitizeInterfaceInlineFragment(ctx *PlanningContext, selectionSet ast.SelectionSet, selection *ast.InlineFragment) ast.SelectionSet {
	possibleTypes := ctx.Schema.PossibleTypes[selection.ObjectDefinition.Name]

	// if it's already inlineFragment witch matches possible type condition just sanitize child fields
	if lo.ContainsBy(possibleTypes, func(d *ast.Definition) bool {
		return d.Name == selection.TypeCondition
	}) {
		selection.SelectionSet = selectionSet
		return ast.SelectionSet{selection}
	}

	for _, pt := range possibleTypes {
		inlineFragment := &ast.InlineFragment{
			TypeCondition:    pt.Name,
			Directives:       selection.Directives,
			SelectionSet:     selection.SelectionSet,
			ObjectDefinition: pt,
		}
		css := &selectionSet
		inlineFragment.SelectionSet = *css
		selectionSet = addSelectionSetToSanitizedResult(selectionSet, inlineFragment)
	}
	return selectionSet
}

func setMissingScrubFieldsForFieldSelectionSet(ctx *PlanningContext, insertionPoint []string, field *ast.Field, scrubFields ScrubFields, addedFields []string) ScrubFields {
	for _, f := range addedFields {
		path := append(insertionPoint, field.Alias)
		if t, _ := ctx.Schema.Types[field.Definition.Type.Name()]; t != nil && (t.Kind == ast.Interface || t.Kind == ast.Union) {
			for _, pt := range ctx.Schema.PossibleTypes[t.Name] {
				scrubFields.Set(path, pt.Name, f)
			}
		} else {
			scrubFields.Set(path, field.Definition.Type.Name(), f)
		}
	}

	return scrubFields
}

func addScrubFieldsToSelectionSet(ctx *PlanningContext, selectionSet ast.SelectionSet, fieldname string) (ast.SelectionSet, []string) {
	var addedFields []string
	var isImplementsNode bool

	if t, _ := ctx.Schema.Types[fieldname]; t != nil && (t.Kind == ast.Interface || t.Kind == ast.Union) {
		pt := ctx.Schema.PossibleTypes[fieldname]
		if !isContainsField(selectionSet, common.TypenameFieldName) {
			selectionSet = addTypenameFieldToSelectionSet(selectionSet)
			addedFields = append(addedFields, common.TypenameFieldName)
		}
		// check that union or interface definition
		// contains ID field AND it's children implements node
		fd := t.Fields.ForName(common.IDFieldName)
		isImplementsNode, _ = ctx.TypeURLMap.GetTypeIsImplementsNode(pt[0].Name)

		isImplementsNode = isImplementsNode && fd != nil

	} else {
		isImplementsNode, _ = ctx.TypeURLMap.GetTypeIsImplementsNode(fieldname)
	}

	if !isImplementsNode {
		return selectionSet, addedFields
	}

	isFoundIDField := isContainsField(selectionSet, common.IDFieldName)

	if isFoundIDField {
		return selectionSet, addedFields
	}

	selectionSet = addIDFieldToSelectionSet(selectionSet)

	addedFields = append(addedFields, common.IDFieldName)

	return selectionSet, addedFields
}

func addSelectionSetToSanitizedResult(s ast.SelectionSet, ss ...ast.Selection) ast.SelectionSet {
	ss = lo.Filter(ss, func(sel ast.Selection, i int) bool {
		f, ok := sel.(*ast.Field)
		if ok && selectionSetHasFieldNamed(s, f.Alias) {
			return false
		}
		return true

	})
	return append(s, ss...)
}
