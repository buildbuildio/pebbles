package merger

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/buildbuildio/pebbles/common"

	"github.com/samber/lo"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
)

type ExtendMergerFunc func(schemas []*MergeInput) (*MergeResult, error)

func (em ExtendMergerFunc) Merge(inputs []*MergeInput) (*MergeResult, error) {
	if len(inputs) < 1 {
		return nil, fmt.Errorf("no source schemas")
	}

	merged := ast.Schema{
		Types:         make(map[string]*ast.Definition),
		Directives:    make(map[string]*ast.DirectiveDefinition),
		PossibleTypes: make(map[string][]*ast.Definition),
	}

	tm := make(TypeURLMap)

	merged.Types = inputs[0].Schema.Types
	tm.SetFromSchema(merged.Types, inputs[0].URL)

	schemas := []*ast.Schema{inputs[0].Schema}

	for i, input := range inputs[1:] {
		mergedTypes, err := mergeTypes(merged.Types, input.Schema.Types, schemas[i], input.Schema)
		if err != nil {
			return nil, err
		}
		tm.SetFromSchema(input.Schema.Types, input.URL)
		merged.Types = mergedTypes

		schemas = append(schemas, input.Schema)
	}

	merged.Implements = mergeImplements(schemas)
	merged.PossibleTypes = mergePossibleTypes(schemas, merged.Types)
	merged.Directives = mergeDirectives(schemas)

	merged.Query = merged.Types[common.QueryObjectName]
	merged.Mutation = merged.Types[common.MutationObjectName]
	merged.Subscription = merged.Types[common.SubscriptionObjectName]

	// sometimes union definition from remote schema is broken so
	// we need to update type def with info stored in possible types
	for name, def := range merged.Types {
		if def.Kind != ast.Union {
			continue
		}

		if len(def.Types) == 0 {
			for _, df := range merged.PossibleTypes[name] {
				def.Types = append(def.Types, df.Name)
			}
		}
	}

	// Reformat schema
	mergedStr := formatSchema(&merged)

	res, err := gqlparser.LoadSchema(&ast.Source{Name: "schema", Input: mergedStr})
	if err != nil {
		return nil, err
	}

	return &MergeResult{Schema: res, TypeURLMap: tm}, nil
}

func mergeTypes(a, b map[string]*ast.Definition, as, bs *ast.Schema) (map[string]*ast.Definition, error) {
	result := make(map[string]*ast.Definition)
	// copy to a to a result
	for k, v := range a {
		nv := *v
		result[k] = &nv
	}

	for k, vb := range b {
		// builin stuff already in result
		if common.IsBuiltinName(k) {
			continue
		}
		nvb := *vb

		va, found := result[k]
		// if not found in a, add to result and continue
		if !found {
			result[k] = &nvb
			continue
		}

		// skip node
		if nvb.Name == common.NodeInterfaceName {
			continue
		}

		if nvb.Kind != va.Kind {
			return nil, fmt.Errorf("name collision: %s(%s) conflicts with %s(%s)", nvb.Name, nvb.Kind, va.Name, va.Kind)
		}

		// if it's scalar just override it
		if nvb.Kind == ast.Scalar {
			result[k] = &nvb
			continue
		}

		// for union all types must be equal
		if nvb.Kind == ast.Union {
			v1, v2 := lo.Difference(va.Types, nvb.Types)
			if len(v1) != 0 || len(v2) != 0 {
				return nil, fmt.Errorf("union collision: %s(%s) conflicting types %v(%v)", va.Name, va.Kind, v1, v2)
			}
			continue
		}

		// for interface all possible types must be equal (except node)
		if nvb.Kind == ast.Interface {
			var anames []string
			for _, v := range as.PossibleTypes[va.Name] {
				anames = append(anames, v.Name)
			}
			var bnames []string
			for _, v := range as.PossibleTypes[nvb.Name] {
				bnames = append(bnames, v.Name)
			}

			v1, v2 := lo.Difference(anames, bnames)
			if len(v1) != 0 || len(v2) != 0 {
				return nil, fmt.Errorf("interface collision: %s(%s) conflicting possible types %v(%v)", va.Name, va.Kind, v1, v2)
			}
		}

		// check that both types implements NodeInterface if one does
		if isImplementsNodeInterface(&nvb) != isImplementsNodeInterface(va) {
			return nil, fmt.Errorf("node interface collision: %s(%s) not implemented in all schemas", nvb.Name, nvb.Kind)
		}

		if common.IsRootObjectName(k) {
			mergedObject, err := mergeRootObjects(a, b, &nvb, va)
			if err != nil {
				return nil, err
			}
			result[k] = mergedObject
			continue
		}

		mergedObject, err := mergeCustomObjects(a, b, &nvb, va)
		if err != nil {
			return nil, err
		}

		result[k] = mergedObject
	}

	return result, nil
}

func mergeImplements(sources []*ast.Schema) map[string][]*ast.Definition {
	result := map[string][]*ast.Definition{}
	for _, schema := range sources {
		for typeName, interfaces := range schema.Implements {
			for _, i := range interfaces {
				result[typeName] = append(result[typeName], i)
			}
		}
	}
	return result
}

func mergeDirectives(sources []*ast.Schema) map[string]*ast.DirectiveDefinition {
	result := map[string]*ast.DirectiveDefinition{}
	for _, schema := range sources {
		for directive, definition := range schema.Directives {
			result[directive] = definition
		}
	}
	return result
}

func mergePossibleTypes(sources []*ast.Schema, mergedTypes map[string]*ast.Definition) map[string][]*ast.Definition {
	result := map[string][]*ast.Definition{}
	for _, schema := range sources {
		for typeName, interfaces := range schema.PossibleTypes {
			if _, ok := mergedTypes[typeName]; !ok {
				continue
			}
			for _, i := range interfaces {
				if ast.DefinitionList(result[typeName]).ForName(i.Name) == nil {
					result[typeName] = append(result[typeName], i)
				}
			}
		}
	}
	return result
}

func mergeRootObjects(aTypes, bTypes map[string]*ast.Definition, a, b *ast.Definition) (*ast.Definition, error) {
	var fields ast.FieldList = a.Fields
	for _, f := range b.Fields {
		if common.IsBuiltinName(f.Name) || isNodeField(f) {
			continue
		}

		if rf := fields.ForName(f.Name); rf != nil {
			return nil, fmt.Errorf("overlapping root types fields %s : %s", a.Name, f.Name)
		}
		fields = append(fields, f)
	}

	return &ast.Definition{
		Kind:        ast.Object,
		Description: mergeDescriptions(a, b),
		Name:        a.Name,
		Directives:  nil,
		Interfaces:  mergeInterfaces(a.Interfaces, b.Interfaces),
		Fields:      fields,
	}, nil
}

func mergeCustomObjects(aTypes, bTypes map[string]*ast.Definition, a, b *ast.Definition) (*ast.Definition, error) {
	result := &ast.Definition{
		Kind:        a.Kind,
		Description: mergeDescriptions(a, b),
		Name:        a.Name,
		Directives:  mergeDirectiveLists(a.Directives, b.Directives),
		Interfaces:  mergeInterfaces(a.Interfaces, b.Interfaces),
		Fields:      nil,
		EnumValues:  mergeEnumValues(a.EnumValues, b.EnumValues),
		Types:       lo.Uniq(append(a.Types, b.Types...)),
	}

	mergedFields, err := mergeCustomObjectFields(aTypes, bTypes, a, b)
	if err != nil {
		return nil, err
	}
	// check if can merge in another order
	if _, err := mergeCustomObjectFields(bTypes, aTypes, b, a); err != nil {
		return nil, err
	}

	result.Fields = mergedFields
	return result, nil
}

func mergeCustomObjectFields(aTypes, bTypes map[string]*ast.Definition, a, b *ast.Definition) (ast.FieldList, error) {
	var result ast.FieldList
	for _, f := range a.Fields {
		if common.IsQueryObjectName(a.Name) && isNodeField(f) {
			continue
		}
		v := *f
		result = append(result, &v)
	}

	var unchnagedResult ast.FieldList
	for _, f := range result {
		v := *f
		unchnagedResult = append(unchnagedResult, &v)
	}

	isOverlappinggMap := make(map[int]bool)
	mf := mergeableFields(b)
	for i, f := range mf {
		if isIDField(f) {
			continue
		}

		rf := result.ForName(f.Name)
		isOverlappinggMap[i] = rf != nil
		result = append(result, f)
	}

	var isSomeOverlappingg bool = false
	var isAllOverlappingg bool = true
	for _, isOverlappingg := range isOverlappinggMap {
		isSomeOverlappingg = isSomeOverlappingg || isOverlappingg
		isAllOverlappingg = isAllOverlappingg && isOverlappingg
	}

	var overlappingFields []string
	for i, isOverlapping := range isOverlappinggMap {
		if !isOverlapping {
			continue
		}
		overlappingFields = append(overlappingFields, mf[i].Name)
	}

	// No overlapping fields for types, which implements node
	if isImplementsNodeInterface(a) && isSomeOverlappingg {
		return nil, fmt.Errorf("overlapping fields %s : %s", a.Name, strings.Join(overlappingFields, ","))
	}

	// not complete copy
	if isSomeOverlappingg && !isAllOverlappingg {
		return nil, fmt.Errorf("overlapping fields, not complete copy %s : %s", a.Name, strings.Join(overlappingFields, ","))
	}

	if isAllOverlappingg {
		return unchnagedResult, nil
	}

	return result, nil
}

func mergeableFields(t *ast.Definition) ast.FieldList {
	result := ast.FieldList{}
	for _, f := range t.Fields {
		if common.IsBuiltinName(f.Name) {
			continue
		}
		result = append(result, f)
	}
	return result
}

func mergeDescriptions(a, b *ast.Definition) string {
	if a.Description == "" {
		return b.Description
	}
	if b.Description == "" {
		return a.Description
	}

	if a.Description == b.Description {
		return a.Description
	}

	return b.Description + "\n\n" + a.Description
}

func mergeEnumValues(a, b ast.EnumValueList) ast.EnumValueList {
	return lo.UniqBy(append(a, b...), func(d *ast.EnumValueDefinition) string {
		return d.Name
	})

}

func mergeInterfaces(a, b []string) []string {
	return lo.Uniq(append(a, b...))
}

func mergeDirectiveLists(a, b ast.DirectiveList) ast.DirectiveList {
	return lo.UniqBy(append(a, b...), func(d *ast.Directive) string {
		return d.Name
	})
}

func formatSchema(schema *ast.Schema) string {
	buf := bytes.NewBufferString("")
	f := formatter.NewFormatter(buf)
	f.FormatSchema(schema)
	return buf.String()
}

func isIDField(f *ast.FieldDefinition) bool {
	return f.Name == common.IDFieldName && len(f.Arguments) == 0 && isIDType(f.Type)
}

func isImplementsNodeInterface(d *ast.Definition) bool {
	return lo.Contains(d.Interfaces, common.NodeInterfaceName)
}

func isIDType(t *ast.Type) bool {
	return isNonNullableTypeNamed(t, "ID")
}

func isNonNullableTypeNamed(t *ast.Type, typename string) bool {
	return t.Name() == typename && t.NonNull
}

func isNullableTypeNamed(t *ast.Type, typename string) bool {
	return t.Name() == typename && !t.NonNull
}

func isNodeField(f *ast.FieldDefinition) bool {
	if common.IsNodeInterfaceName(f.Name) || len(f.Arguments) != 1 {
		return false
	}
	arg := f.Arguments[0]
	return arg.Name == common.IDFieldName &&
		isIDType(arg.Type) &&
		isNullableTypeNamed(f.Type, common.NodeInterfaceName)
}
