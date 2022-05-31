package introspection

import (
	"sort"
	"strings"

	"github.com/buildbuildio/pebbles/common"

	"github.com/vektah/gqlparser/v2/ast"
)

type IntrospectionResolver struct {
	Variables map[string]interface{}
}

func (ir *IntrospectionResolver) ResolveIntrospectionFields(selectionSet ast.SelectionSet, schema *ast.Schema) map[string]interface{} {
	introspectionResult := make(map[string]interface{})
	var isIntrospection bool
	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "__type":
			name := f.Arguments.ForName("name").Value.Raw
			introspectionResult[f.Alias] = ir.resolveType(schema, &ast.Type{NamedType: name}, f.SelectionSet)
			isIntrospection = true
		case "__schema":
			introspectionResult[f.Alias] = ir.resolveSchema(schema, f.SelectionSet)
			isIntrospection = true
		}
	}

	if !isIntrospection {
		return nil
	}

	return introspectionResult
}

func (ir *IntrospectionResolver) resolveSchema(schema *ast.Schema, selectionSet ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "types":
			types := []map[string]interface{}{}
			for _, t := range schema.Types {
				types = append(types, ir.resolveType(schema, &ast.Type{NamedType: t.Name}, f.SelectionSet))
			}
			sortPayload(types)
			result[f.Alias] = types
		case "queryType":
			result[f.Alias] = ir.resolveType(schema, &ast.Type{NamedType: "Query"}, f.SelectionSet)
		case "mutationType":
			result[f.Alias] = ir.resolveType(schema, &ast.Type{NamedType: "Mutation"}, f.SelectionSet)
		case "subscriptionType":
			result[f.Alias] = ir.resolveType(schema, &ast.Type{NamedType: "Subscription"}, f.SelectionSet)
		case "directives":
			directives := []map[string]interface{}{}
			for _, d := range schema.Directives {
				directives = append(directives, ir.resolveDirective(schema, d, f.SelectionSet))
			}
			sortPayload(directives)
			result[f.Alias] = directives
		}
	}

	return result
}

func (ir *IntrospectionResolver) resolveType(schema *ast.Schema, typ *ast.Type, selectionSet ast.SelectionSet) map[string]interface{} {
	if typ == nil {
		return nil
	}

	result := make(map[string]interface{})

	// If the type is NON_NULL or LIST then use that first (in that order), then
	// recursively call in "ofType"

	if typ.NonNull {
		for _, f := range common.SelectionSetToFields(selectionSet, nil) {
			switch f.Name {
			case "kind":
				result[f.Alias] = "NON_NULL"
			case "ofType":
				result[f.Alias] = ir.resolveType(schema, &ast.Type{
					NamedType: typ.NamedType,
					Elem:      typ.Elem,
					NonNull:   false,
				}, f.SelectionSet)
			default:
				result[f.Alias] = nil
			}
		}
		return result
	}

	if typ.Elem != nil {
		for _, f := range common.SelectionSetToFields(selectionSet, nil) {
			switch f.Name {
			case "kind":
				result[f.Alias] = "LIST"
			case "ofType":
				result[f.Alias] = ir.resolveType(schema, typ.Elem, f.SelectionSet)
			default:
				result[f.Alias] = nil
			}
		}
		return result
	}

	namedType, ok := schema.Types[typ.NamedType]
	if !ok {
		return nil
	}

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "kind":
			result[f.Alias] = namedType.Kind
		case "name":
			result[f.Alias] = namedType.Name
		case "fields":
			includeDeprecated := false
			if deprecatedArg := f.Arguments.ForName("includeDeprecated"); deprecatedArg != nil {
				v, err := deprecatedArg.Value.Value(ir.Variables)
				if err == nil {
					includeDeprecated, _ = v.(bool)
				}
			}

			fields := []map[string]interface{}{}
			for _, fi := range namedType.Fields {
				if common.IsBuiltinName(fi.Name) {
					continue
				}
				if !includeDeprecated {
					if deprecated, _ := hasDeprecatedDirective(fi.Directives); deprecated {
						continue
					}
				}
				fields = append(fields, ir.resolveField(schema, fi, f.SelectionSet))
			}
			result[f.Alias] = fields
		case "description":
			result[f.Alias] = namedType.Description
		case "interfaces":
			interfaces := []map[string]interface{}{}
			for _, i := range namedType.Interfaces {
				interfaces = append(interfaces, ir.resolveType(schema, &ast.Type{NamedType: i}, f.SelectionSet))
			}
			result[f.Alias] = interfaces
		case "possibleTypes":
			if len(namedType.Types) > 0 {
				types := []map[string]interface{}{}
				for _, t := range namedType.Types {
					types = append(types, ir.resolveType(schema, &ast.Type{NamedType: t}, f.SelectionSet))
				}
				result[f.Alias] = types
			} else {
				result[f.Alias] = nil
			}
		case "enumValues":
			includeDeprecated := false
			if deprecatedArg := f.Arguments.ForName("includeDeprecated"); deprecatedArg != nil {
				v, err := deprecatedArg.Value.Value(ir.Variables)
				if err == nil {
					includeDeprecated, _ = v.(bool)
				}
			}

			enums := []map[string]interface{}{}
			for _, e := range namedType.EnumValues {
				if !includeDeprecated {
					if deprecated, _ := hasDeprecatedDirective(e.Directives); deprecated {
						continue
					}
				}
				enums = append(enums, resolveEnumValue(e, f.SelectionSet))
			}
			result[f.Alias] = enums
		case "inputFields":
			inputFields := []map[string]interface{}{}
			for _, fi := range namedType.Fields {
				// call resolveField instead of resolveInputValue because it has
				// the right type and is a superset of it
				inputFields = append(inputFields, ir.resolveField(schema, fi, f.SelectionSet))
			}
			result[f.Alias] = inputFields
		default:
			result[f.Alias] = nil
		}
	}

	return result
}

func (ir *IntrospectionResolver) resolveField(schema *ast.Schema, field *ast.FieldDefinition, selectionSet ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	deprecated, deprecatedReason := hasDeprecatedDirective(field.Directives)

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "name":
			result[f.Alias] = field.Name
		case "description":
			result[f.Alias] = field.Description
		case "args":
			args := []map[string]interface{}{}
			for _, arg := range field.Arguments {
				args = append(args, ir.resolveInputValue(schema, arg, f.SelectionSet))
			}
			result[f.Alias] = args
		case "type":
			result[f.Alias] = ir.resolveType(schema, field.Type, f.SelectionSet)
		case "isDeprecated":
			result[f.Alias] = deprecated
		case "deprecationReason":
			result[f.Alias] = deprecatedReason
		}
	}

	return result
}

func (ir *IntrospectionResolver) resolveDirective(schema *ast.Schema, directive *ast.DirectiveDefinition, selectionSet ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "name":
			result[f.Alias] = directive.Name
		case "description":
			result[f.Alias] = directive.Description
		case "locations":
			result[f.Alias] = directive.Locations
		case "args":
			args := []map[string]interface{}{}
			for _, arg := range directive.Arguments {
				args = append(args, ir.resolveInputValue(schema, arg, f.SelectionSet))
			}
			result[f.Alias] = args
		}
	}

	return result
}

func hasDeprecatedDirective(directives ast.DirectiveList) (bool, *string) {
	for _, d := range directives {
		if d.Name == "deprecated" {
			var reason string
			reasonArg := d.Arguments.ForName("reason")
			if reasonArg != nil {
				reason = reasonArg.Value.Raw
			}
			return true, &reason
		}
	}

	return false, nil
}

func (ir *IntrospectionResolver) resolveInputValue(schema *ast.Schema, arg *ast.ArgumentDefinition, selectionSet ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "name":
			result[f.Alias] = arg.Name
		case "description":
			result[f.Alias] = arg.Description
		case "type":
			result[f.Alias] = ir.resolveType(schema, arg.Type, f.SelectionSet)
		case "defaultValue":
			if arg.DefaultValue != nil {
				result[f.Alias] = arg.DefaultValue.String()
			} else {
				result[f.Alias] = nil
			}
		}
	}

	return result
}

func resolveEnumValue(enum *ast.EnumValueDefinition, selectionSet ast.SelectionSet) map[string]interface{} {
	result := make(map[string]interface{})

	deprecated, deprecatedReason := hasDeprecatedDirective(enum.Directives)

	for _, f := range common.SelectionSetToFields(selectionSet, nil) {
		switch f.Name {
		case "name":
			result[f.Alias] = enum.Name
		case "description":
			result[f.Alias] = enum.Description
		case "isDeprecated":
			result[f.Alias] = deprecated
		case "deprecationReason":
			result[f.Alias] = deprecatedReason
		}
	}

	return result
}

func sortPayload(payload []map[string]interface{}) {
	sort.SliceStable(payload, func(i, j int) bool {
		left, lok := payload[i]["name"].(string)
		right, rok := payload[j]["name"].(string)
		if !lok || !rok {
			return false
		}
		return strings.Compare(left, right) < 0
	})

}
