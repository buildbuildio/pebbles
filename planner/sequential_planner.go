package planner

import (
	"fmt"

	"github.com/buildbuildio/pebbles/common"

	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/ast"
)

type SequentialPlanner func(ctx *PlanningContext) (*QueryPlan, error)

// Plan returns a query plan from the given planning context
func (sp SequentialPlanner) Plan(ctx *PlanningContext) (*QueryPlan, error) {
	var parentType string
	switch ctx.Operation.Operation {
	case ast.Query:
		parentType = common.QueryObjectName
	case ast.Mutation:
		parentType = common.MutationObjectName
	default:
		return nil, fmt.Errorf("not implemented")
	}

	selSet, sf := sanitizeSelectionSet(ctx, ctx.Operation.SelectionSet, nil)
	if len(sf) == 0 {
		sf = nil
	}

	steps, err := createQueryPlanSteps(ctx, nil, parentType, "", selSet)
	if err != nil {
		return nil, err
	}

	qp := &QueryPlan{
		RootSteps:   steps,
		ScrubFields: sf,
	}

	return qp.SetComputedValues(ctx), nil
}

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
				for _, f := range addedFields {
					path := append(insertionPoint, s.Alias)
					if t, _ := ctx.Schema.Types[s.Definition.Type.Name()]; t != nil && (t.Kind == ast.Interface || t.Kind == ast.Union) {
						for _, pt := range ctx.Schema.PossibleTypes[t.Name] {
							scrubFields.Set(path, pt.Name, f)
						}
					} else {
						scrubFields.Set(path, s.Definition.Type.Name(), f)
					}
				}

				s.SelectionSet = childSelectionSet
			}
			result = append(result, s)
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
			result = append(result, selSet...)
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
				possibleTypes := ctx.Schema.PossibleTypes[s.ObjectDefinition.Name]

				// if it's already inlineFragment witch matches possible type condition just sanitize child fields
				if lo.ContainsBy(possibleTypes, func(d *ast.Definition) bool {
					return d.Name == s.TypeCondition
				}) {
					s.SelectionSet = childSelectionSet
					result = append(result, s)
					continue
				}

				for _, pt := range possibleTypes {
					inlineFragment := &ast.InlineFragment{
						TypeCondition:    pt.Name,
						Directives:       s.Directives,
						SelectionSet:     s.SelectionSet,
						ObjectDefinition: pt,
					}
					css := &childSelectionSet
					inlineFragment.SelectionSet = *css
					childSelectionSet = append(childSelectionSet, inlineFragment)
				}
				result = append(result, childSelectionSet...)
			case ast.Union:
				s.SelectionSet = childSelectionSet
				result = append(result, s)
			default:
				result = append(result, childSelectionSet...)
			}

		}
	}

	return result, scrubFields
}

func createQueryPlanSteps(ctx *PlanningContext, insertionPoint []string, parentType, parentLocation string, selectionSet ast.SelectionSet) ([]*QueryPlanStep, error) {
	var result []*QueryPlanStep

	routedSelectionSet, err := routeSelectionSet(ctx, parentType, parentLocation, selectionSet)
	if err != nil {
		return nil, err
	}

	for location, selectionSet := range routedSelectionSet {
		selectionSetForLocation, childrenSteps, err := extractSelectionSet(ctx, insertionPoint, parentType, selectionSet, location)
		if err != nil {
			return nil, err
		}

		// the insertionPoint slice can be modified later as we're appending
		// values to it while recursively traversing the selection set, so we
		// need to make a copy
		var insertionPointCopy []string
		if len(insertionPoint) > 0 {
			insertionPointCopy = make([]string, len(insertionPoint))
			copy(insertionPointCopy, insertionPoint)
		}

		qps := &QueryPlanStep{
			InsertionPoint: insertionPointCopy,
			Then:           childrenSteps,
			URL:            location,
			ParentType:     parentType,
			SelectionSet:   selectionSetForLocation,
		}

		result = append(result, qps)
	}
	return result, nil
}

func extractSelectionSet(ctx *PlanningContext, insertionPoint []string, parentType string, input ast.SelectionSet, location string) (ast.SelectionSet, []*QueryPlanStep, error) {
	var selectionSetResult []ast.Selection
	var childrenStepsResult []*QueryPlanStep

	isParentTypeImplementsNode, _ := ctx.TypeURLMap.GetTypeIsImplementsNode(parentType)

	// check that parentType is interface, which types spread across multiple services
	parentTypeDef, ok := ctx.Schema.Types[parentType]
	if !ok {
		return nil, nil, fmt.Errorf("unable to find type %s in schema", parentType)
	}

	if parentTypeDef.Kind == ast.Interface {
		input = formatSelectionSetForInterface(ctx, insertionPoint, parentType, input, location)
	}

	for _, selection := range input {
		switch selection := selection.(type) {
		case *ast.Field:
			loc, err := ctx.GetURL(parentType, selection.Name, location)
			if err != nil {
				// f.e. here can be fields of interfaces or id fields, just add them straight to selection
				selectionSetResult = append(selectionSetResult, selection)
				continue
			}
			if loc == location {
				if selection.SelectionSet == nil {
					selectionSetResult = append(selectionSetResult, selection)
				} else {
					newField := *selection
					selectionSet, childrenSteps, err := extractSelectionSet(
						ctx,
						append(insertionPoint, selection.Alias),
						selection.Definition.Type.Name(),
						selection.SelectionSet,
						location,
					)
					if err != nil {
						return nil, nil, err
					}

					newField.SelectionSet = selectionSet
					selectionSetResult = append(selectionSetResult, &newField)
					childrenStepsResult = append(childrenStepsResult, childrenSteps...)
				}
			} else {
				mergedWithExistingStep := false
				for _, step := range childrenStepsResult {
					if step.URL == loc && common.IsEqual(step.InsertionPoint, insertionPoint) {
						// add to node query
						s, ok := addFieldToNodeQuery(parentType, step.SelectionSet, selection)
						if ok {
							step.SelectionSet = s
						} else {
							step.SelectionSet = append(step.SelectionSet, selection)
						}

						mergedWithExistingStep = true
						break
					}
				}

				if !mergedWithExistingStep {
					newSelectionSet := []ast.Selection{selection}
					childrenSteps, err := createQueryPlanSteps(ctx, insertionPoint, parentType, location, newSelectionSet)
					if err != nil {
						return nil, nil, err
					}
					childrenStepsResult = append(childrenStepsResult, childrenSteps...)
				}
			}
		case *ast.InlineFragment:
			selectionSet, childrenSteps, err := extractSelectionSet(
				ctx,
				insertionPoint,
				selection.TypeCondition,
				selection.SelectionSet,
				location,
			)
			if err != nil {
				return nil, nil, err
			}

			inlineFragment := *selection
			inlineFragment.SelectionSet = selectionSet
			selectionSetResult = append(selectionSetResult, &inlineFragment)
			childrenStepsResult = append(childrenStepsResult, childrenSteps...)
		default:
			return nil, nil, fmt.Errorf("unexpected %T in SelectionSet", selection)
		}
	}

	// if we are not querying the top level then we have to embed the selection set
	// under the node query with the right id as the argument
	if !common.IsRootObjectName(parentType) && isParentTypeImplementsNode && !selectionSetHasFieldNamed(selectionSetResult, common.IDFieldName) {
		selectionSetResult = convertSelectionSetToNodeQuery(parentType, selectionSetResult)
	}

	return selectionSetResult, childrenStepsResult, nil
}

func formatSelectionSetForInterface(ctx *PlanningContext, insertionPoint []string, parentType string, selectionSet ast.SelectionSet, location string) ast.SelectionSet {
	defs := ctx.Schema.PossibleTypes[parentType]

	// determine which services contains provided fields
	var urls []string

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		for _, def := range defs {
			us, ok := ctx.TypeURLMap.Get(def.Name, f.Name)
			if !ok {
				continue
			}
			urls = lo.Uniq(append(urls, us))
		}
	}

	// all fields from one service and location matches, no need to do anything
	if len(urls) == 1 && urls[0] == location {
		return selectionSet
	}

	// spread across multiple services, need to query each one
	// we update selection set
	// from { interface { field } }
	// to   { interface { ... on impl { id field } }}

	resSelectionSet := addTypenameFieldToSelectionSet(nil)
	for _, def := range defs {
		// remove fragments and inline fragment for specific definition
		fieldsSelSet := selectionSetToFieldsRepresentation(selectionSet, def)

		inlineFragment := ast.InlineFragment{
			TypeCondition: def.Name,
			SelectionSet:  fieldsSelSet,
		}
		resSelectionSet = append(resSelectionSet, &inlineFragment)
	}

	return resSelectionSet
}

func routeSelectionSet(ctx *PlanningContext, parentType, parentLocation string, input ast.SelectionSet) (map[string]ast.SelectionSet, error) {
	result := map[string]ast.SelectionSet{}
	if parentLocation == "" {
		// if we're at the root, we extract the selection set for each service
		for _, loc := range ctx.TypeURLMap.GetURLs() {
			ss, err := filterSelectionSetByLoc(ctx, input, loc, parentType)
			if err != nil {
				return nil, err
			}
			if len(ss) > 0 {
				result[loc] = ss
			}
		}

		if ss, err := filterSelectionSetByLoc(ctx, input, common.InternalServiceName, parentType); err == nil && len(ss) > 0 {
			result[common.InternalServiceName] = ss
		}

		return result, nil
	}

	for _, selection := range input {
		switch selection := selection.(type) {
		case *ast.Field:
			if common.IsBuiltinName(selection.Name) && parentLocation == "" {
				continue
			}
			var loc string
			if selection.Name == common.TypenameFieldName {
				loc = parentLocation
			} else {
				var ok bool
				loc, ok = ctx.TypeURLMap.Get(parentType, selection.Name)
				if !ok {
					return nil, fmt.Errorf("could not find location for %s", selection.Name)
				}
			}

			result[loc] = append(result[loc], selection)
		default:
			return nil, fmt.Errorf("unexpected selection type: %T", selection)
		}
	}
	return result, nil
}

func selectionSetToFieldsRepresentation(selectionSet ast.SelectionSet, parentDef *ast.Definition) ast.SelectionSet {
	fields := common.SelectionSetToFields(selectionSet, parentDef)

	uniq := make(map[string]struct{})
	var res ast.SelectionSet
	// save only uniq aliases/names for fields
	// it happens when selection set contains multiple implementations
	// for interfaces or union
	for _, f := range fields {
		fName := f.Alias
		if fName == "" {
			fName = f.Name
		}
		if _, ok := uniq[fName]; ok {
			continue
		}
		uniq[fName] = struct{}{}
		res = append(res, f)
	}
	return res
}

func filterSelectionSetByLoc(ctx *PlanningContext, ss ast.SelectionSet, loc, parentType string) (ast.SelectionSet, error) {
	var res ast.SelectionSet

	for _, selection := range common.SelectionSetToFields(ss, nil) {
		fieldLoc, err := ctx.GetURL(parentType, selection.Name, common.InternalServiceName)
		if err != nil {
			return nil, err
		}

		if fieldLoc == loc {
			res = append(res, selection)
		}
	}

	return res, nil
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

func addIDFieldToSelectionSet(selectionSet ast.SelectionSet) ast.SelectionSet {
	return append(ast.SelectionSet{&ast.Field{
		Name: common.IDFieldName,
		Definition: &ast.FieldDefinition{
			Type: &ast.Type{
				NamedType: common.IDFieldName,
				NonNull:   true,
			},
		},
	}}, selectionSet...)
}

func addTypenameFieldToSelectionSet(selectionSet ast.SelectionSet) ast.SelectionSet {
	return append(ast.SelectionSet{&ast.Field{
		Name: common.TypenameFieldName,
		Definition: &ast.FieldDefinition{
			Type: &ast.Type{
				NamedType: "String",
			},
		},
	}}, selectionSet...)
}

func isContainsField(selectionSet ast.SelectionSet, fieldname string) bool {
	for _, selection := range selectionSet {
		switch sel := selection.(type) {
		case *ast.Field:
			if sel.Name == fieldname {
				return true
			}
		case *ast.InlineFragment:
			if isContainsField(sel.SelectionSet, fieldname) {
				return true
			}
		default:
			continue
		}
	}

	return false
}

// convertSelectionSetToNode converts selection set and parent type to this
// {
//	 	node(id: $id) {
//	 		... on parentType {
//	 			selectionSet
//	 		}
//	 	}
// }
func convertSelectionSetToNodeQuery(parentType string, selectionSet ast.SelectionSet) ast.SelectionSet {
	return ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: common.IDFieldName,
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  common.IDFieldName,
					},
				},
			},
			Definition: &ast.FieldDefinition{
				Name: common.NodeFieldName,
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: common.IDFieldName,
						Type: ast.NamedType("ID!", nil),
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.InlineFragment{
					TypeCondition: parentType,
					SelectionSet:  selectionSet,
				},
			},
		},
	}
}

// addFieldToNodeQuery adds provided selection to node query
// {
//	 	node(id: $id) {
//	 		... on parentType {
//	 			...existingSelectionSet
//				selection (*)
//	 		}
//	 	}
// }
func addFieldToNodeQuery(parentType string, nodeQuery ast.SelectionSet, selection ast.Selection) (ast.SelectionSet, bool) {
	if len(nodeQuery) == 0 {
		return nil, false
	}
	nodeSpread, ok := nodeQuery[0].(*ast.Field)
	if !ok || len(nodeSpread.SelectionSet) == 0 || nodeSpread.Name != common.NodeFieldName {
		return nil, false
	}

	spreadFields, ok := nodeSpread.SelectionSet[0].(*ast.InlineFragment)
	if !ok {
		return nil, false
	}

	return convertSelectionSetToNodeQuery(parentType, append(spreadFields.SelectionSet, selection)), true
}

func selectionSetHasFieldNamed(ss []ast.Selection, fieldname string) bool {
	for _, selection := range ss {
		field, ok := selection.(*ast.Field)
		if ok && field.Name == fieldname {
			return true
		}
	}
	return false
}
