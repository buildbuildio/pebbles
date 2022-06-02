package introspection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/formatter"
	"golang.org/x/exp/slices"
)

var introspectionQueryName string = "IntrospectionQuery"

type RemoteSchemaIntrospector interface {
	IntrospectRemoteSchemas(...string) ([]*ast.Schema, error)
}

type ParallelRemoteSchemaIntrospector struct {
	Factory QueryerFactory
}

var _ RemoteSchemaIntrospector = &ParallelRemoteSchemaIntrospector{}

type QueryerFactory func(string) queryer.Queryer

func (p *ParallelRemoteSchemaIntrospector) IntrospectRemoteSchemas(urls ...string) ([]*ast.Schema, error) {
	type inner struct {
		schema *ast.Schema
		index  int
	}

	var acc []*inner

	res, err := common.AsyncMapReduce(lo.Range(len(urls)), acc, func(i int) (*inner, error) {
		res, err := introspectRemoteSchema(p.Factory, urls[i])
		if err != nil {
			return nil, err
		}
		return &inner{schema: res, index: i}, nil
	}, func(acc []*inner, value *inner) []*inner {
		return append(acc, value)
	})

	if err != nil {
		return nil, err
	}

	slices.SortStableFunc(res, func(a, b *inner) bool {
		return a.index < b.index
	})

	return lo.Map(res, func(t *inner, _ int) *ast.Schema { return t.schema }), nil
}

func introspectRemoteSchema(factory QueryerFactory, url string) (*ast.Schema, error) {
	queryer := factory(url)
	resp, err := queryer.Query([]*requests.Request{{
		Query:         introspectionQuery,
		OperationName: &introspectionQueryName,
	}})
	if err != nil {
		return nil, err
	}

	res, err := parseQueryerResponse(resp)
	if err != nil {
		return nil, err
	}

	remoteSchema := res.Schema

	schema := &ast.Schema{
		Types:         map[string]*ast.Definition{},
		Directives:    map[string]*ast.DirectiveDefinition{},
		PossibleTypes: map[string][]*ast.Definition{},
		Implements:    map[string][]*ast.Definition{},
	}

	if remoteSchema == nil || remoteSchema.QueryType.Name == "" {
		return nil, errors.New("could not find the root query")
	}

	for _, remoteType := range remoteSchema.Types {
		// convert turn the API payload into a schema type
		schemaType := parseType(remoteType)
		if schemaType == nil {
			continue
		}

		// check if this type is the QueryType
		if remoteType.Name == remoteSchema.QueryType.Name {
			schema.Query = schemaType
		} else if remoteSchema.MutationType != nil && schemaType.Name == remoteSchema.MutationType.Name {
			schema.Mutation = schemaType
		} else if remoteSchema.SubscriptionType != nil && schemaType.Name == remoteSchema.SubscriptionType.Name {
			schema.Subscription = schemaType
		}

		// register the type with the schema
		schema.Types[schemaType.Name] = schemaType

		// make sure we record that a type implements itself
		schema.AddImplements(remoteType.Name, schemaType)
	}

	for _, remoteType := range remoteSchema.Types {
		schemaType := schema.Types[remoteType.Name]
		if schemaType == nil {
			continue
		}

		// if we are looking at an enum or union
		// each union value needs to be added to the list
		for _, possibleType := range remoteType.PossibleTypes {
			// if there is no name
			if possibleType.Name == "" {
				return nil, errors.New("could not find type's name")
			}

			// add the type to the union definition
			if remoteType.Name != schemaType.Name {
				schemaType.Types = append(schemaType.Types, possibleType.Name)
			}

			possibleTypeDef, ok := schema.Types[possibleType.Name]
			if !ok {
				return nil, errors.New("could not find type definition for union implementation")
			}

			// add the possible type to the schema
			schema.AddPossibleType(remoteType.Name, possibleTypeDef)
			schema.AddImplements(possibleType.Name, schemaType)
		}

		// each interface value needs to be added to the list
		for _, iface := range remoteType.Interfaces {
			// if there is no name
			if iface.Name == "" {
				return nil, errors.New("could not find type's name")
			}

			// add the type to the union definition
			schemaType.Interfaces = append(schemaType.Interfaces, iface.Name)

			ifaceDef, ok := schema.Types[iface.Name]
			if !ok {
				return nil, errors.New("Could not find type definition for union implementation")
			}

			// add the possible type to the schema
			schema.AddPossibleType(ifaceDef.Name, schemaType)
			schema.AddImplements(schemaType.Name, ifaceDef)
		}

		// we need to update type def with info stored in possible types for unions
		if schemaType.Kind == ast.Union && len(schemaType.Types) == 0 {
			for _, df := range schema.PossibleTypes[schemaType.Name] {
				schemaType.Types = append(schemaType.Types, df.Name)
			}

		}
	}

	// add each directive to the schema
	for _, directive := range remoteSchema.Directives {
		switch directive.Name {
		case "":
			return nil, errors.New("could not find directive's name")
		case "skip", "deprecated", "include":
			// skip builtin stuff, it'll be lately added by gqlparser
			continue
		}

		// the list of directive locations
		var locations []ast.DirectiveLocation
		for _, value := range directive.Locations {
			locations = append(locations, ast.DirectiveLocation(value))
		}

		// save the directive definition to the schema
		schema.Directives[directive.Name] = &ast.DirectiveDefinition{
			// otherwise gqlparser will fail
			Position:    &ast.Position{Src: &ast.Source{}},
			Name:        directive.Name,
			Description: directive.Description,
			Arguments:   parseArgList(directive.Args),
			Locations:   locations,
		}

	}

	// Reformat schema
	schemaStr := formatSchema(schema)

	formattedSchema, perr := gqlparser.LoadSchema(&ast.Source{Name: url, Input: schemaStr})
	if perr != nil {
		return nil, perr
	}

	return formattedSchema, nil
}

func formatSchema(schema *ast.Schema) string {
	buf := bytes.NewBufferString("")
	f := formatter.NewFormatter(buf)
	f.FormatSchema(schema)
	return buf.String()
}

func parseType(remoteType IntrospectionQueryFullType) *ast.Definition {
	switch remoteType.Name {
	// skip builtin stuff, it'll be lately added by gqlparser
	case "ID", "Int", "Float", "String", "Boolean",
		"__Schema", "__Type", "__InputValue", "__TypeKind",
		"__DirectiveLocation", "__Field", "__EnumValue", "__Directive":
		return nil
	}

	definition := &ast.Definition{
		Name:        remoteType.Name,
		Description: remoteType.Description,
	}

	switch remoteType.Kind {
	case "OBJECT":
		definition.Kind = ast.Object
	case "SCALAR":
		definition.Kind = ast.Scalar
	case "INTERFACE":
		definition.Kind = ast.Interface
	case "UNION":
		definition.Kind = ast.Union
	case "INPUT_OBJECT":
		definition.Kind = ast.InputObject
	case "ENUM":
		definition.Kind = ast.Enum

		for _, value := range remoteType.EnumValues {
			definition.EnumValues = append(definition.EnumValues, &ast.EnumValueDefinition{
				Name:        value.Name,
				Description: value.Description,
			})
		}
	}

	// build up a list of fields associated with the type
	var fields ast.FieldList

	for _, field := range remoteType.Fields {
		// add the field to the list
		fields = append(fields, &ast.FieldDefinition{
			Name:        field.Name,
			Type:        parseTypeRef(&field.Type),
			Description: field.Description,
			Arguments:   parseArgList(field.Args),
		})
	}

	for _, field := range remoteType.InputFields {
		// add the field to the list
		fields = append(fields, parseInputField(field))
	}

	definition.Fields = fields

	return definition
}

func parseInputField(field IntrospectionInputValue) *ast.FieldDefinition {
	fd := &ast.FieldDefinition{
		Name:        field.Name,
		Type:        parseTypeRef(&field.Type),
		Description: field.Description,
	}
	if field.DefaultValue == nil {
		return fd
	}

	bRaw, err := json.Marshal(field.DefaultValue)
	if err != nil {
		return fd
	}

	isArray := fd.Type.Elem != nil
	kindStr := fd.Type.Name()

	var vKind ast.ValueKind

	switch kindStr {
	case "Int":
		vKind = ast.IntValue
	case "Float":
		vKind = ast.FloatValue
	case "Boolean":
		vKind = ast.BooleanValue
	default:
		vKind = ast.StringValue
	}

	if isArray {
		arr, ok := field.DefaultValue.([]interface{})
		if !ok {
			return fd
		}

		var children ast.ChildValueList

		for _, el := range arr {
			elRaw, err := json.Marshal(el)
			if err != nil {
				return fd
			}
			if vKind == ast.StringValue && len(elRaw) > 2 {
				// stash additional "" after json marshalling
				elRaw = elRaw[1 : len(elRaw)-1]
			}

			children = append(children, &ast.ChildValue{
				Value: &ast.Value{
					Position: &ast.Position{},
					Raw:      string(elRaw),
					Kind:     vKind,
				},
			})
		}

		fd.DefaultValue = &ast.Value{
			Position: &ast.Position{},
			Kind:     ast.ListValue,
			Children: children,
		}
		return fd
	}

	if vKind == ast.StringValue && len(bRaw) > 2 {
		// stash additional "" after json marshalling
		bRaw = bRaw[1 : len(bRaw)-1]
	}

	fd.DefaultValue = &ast.Value{
		Position: &ast.Position{},
		Raw:      string(bRaw),
		Kind:     vKind,
	}

	return fd
}

func parseArgList(args []IntrospectionInputValue) ast.ArgumentDefinitionList {
	result := ast.ArgumentDefinitionList{}

	// we need to add each argument to the field
	for _, argument := range args {
		result = append(result, &ast.ArgumentDefinition{
			Name:        argument.Name,
			Description: argument.Description,
			Type:        parseTypeRef(&argument.Type),
		})
	}

	return result
}

func parseTypeRef(response *IntrospectionTypeRef) *ast.Type {
	// we could have a non-null list of a field
	if response.Kind == "NON_NULL" && response.OfType.Kind == "LIST" {
		return ast.NonNullListType(parseTypeRef(response.OfType.OfType), &ast.Position{})
	}

	// we could have a list of a type
	if response.Kind == "LIST" {
		return ast.ListType(parseTypeRef(response.OfType), &ast.Position{})
	}

	// we could have just a non null
	if response.Kind == "NON_NULL" {
		return ast.NonNullNamedType(response.OfType.Name, &ast.Position{})
	}

	// if we are looking at a named type that isn't in a list or marked non-null
	return ast.NamedType(response.Name, &ast.Position{})
}

func parseQueryerResponse(resp []map[string]interface{}) (*IntrospectionQueryResult, error) {
	if len(resp) != 1 {
		return nil, errors.New("wrong response length")
	}

	tmp, err := json.Marshal(resp[0])
	if err != nil {
		return nil, err
	}

	var qRes *IntrospectionQueryResult
	if err := json.Unmarshal(tmp, &qRes); err != nil {
		return nil, err
	}

	return qRes, nil

}

type IntrospectionQueryResult struct {
	Schema *IntrospectionQuerySchema `json:"__schema"`
}

type IntrospectionQuerySchema struct {
	QueryType        IntrospectionQueryRootType    `json:"queryType"`
	MutationType     *IntrospectionQueryRootType   `json:"mutationType"`
	SubscriptionType *IntrospectionQueryRootType   `json:"subscriptionType"`
	Types            []IntrospectionQueryFullType  `json:"types"`
	Directives       []IntrospectionQueryDirective `json:"directives"`
}

type IntrospectionQueryDirective struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Locations   []string                  `json:"locations"`
	Args        []IntrospectionInputValue `json:"arg"`
}

type IntrospectionQueryRootType struct {
	Name string `json:"name"`
}

type IntrospectionQueryFullTypeField struct {
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	Args              []IntrospectionInputValue `json:"args"`
	Type              IntrospectionTypeRef      `json:"type"`
	IsDeprecated      bool                      `json:"isDeprecated"`
	DeprecationReason string                    `json:"deprecationReason"`
}

type IntrospectionQueryFullType struct {
	Kind          string                             `json:"kind"`
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	InputFields   []IntrospectionInputValue          `json:"inputFields"`
	Interfaces    []IntrospectionTypeRef             `json:"interfaces"`
	PossibleTypes []IntrospectionTypeRef             `json:"possibleTypes"`
	Fields        []IntrospectionQueryFullTypeField  `json:"fields"`
	EnumValues    []IntrospectionQueryEnumDefinition `json:"enumValues"`
}

type IntrospectionQueryEnumDefinition struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	IsDeprecated      bool   `json:"isDeprecated"`
	DeprecationReason string `json:"deprecationReason"`
}

type IntrospectionInputValue struct {
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	DefaultValue interface{}          `json:"defaultValue"`
	Type         IntrospectionTypeRef `json:"type"`
}

type IntrospectionTypeRef struct {
	Kind   string                `json:"kind"`
	Name   string                `json:"name"`
	OfType *IntrospectionTypeRef `json:"ofType"`
}

var introspectionQuery = fmt.Sprintf(`
query %s {
	__schema {
		queryType { name }
		mutationType { name }
		subscriptionType { name }
		types {
			...FullType
		}
		directives {
			name
			description
			locations
			args {
				...InputValue
			}
		}
	}
}

fragment FullType on __Type {
	kind
	name
	description
	fields(includeDeprecated: true) {
		name
		description
		args {
			...InputValue
		}
		type {
			...TypeRef
		}
		isDeprecated
		deprecationReason
	}

	inputFields {
		...InputValue
	}

	interfaces {
		...TypeRef
	}

	enumValues(includeDeprecated: true) {
		name
		description
		isDeprecated
		deprecationReason
	}
	possibleTypes {
		...TypeRef
	}
}

fragment InputValue on __InputValue {
	name
	description
	type { ...TypeRef }
	defaultValue
}

fragment TypeRef on __Type {
	kind
	name
	ofType {
		kind
		name
		ofType {
			kind
			name
			ofType {
				kind
				name
				ofType {
					kind
					name
					ofType {
						kind
						name
						ofType {
							kind
							name
							ofType {
								kind
								name
							}
						}
					}
				}
			}
		}
	}
}
`, introspectionQueryName)
