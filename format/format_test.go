package format

import (
	"testing"

	"github.com/buildbuildio/pebbles/common"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

var testFormatter = NewDebugBufferedFormatter()

func TestNewBufferedFormatter(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	tf := NewBufferedFormatter()

	tf.WithIndent("XXX")

	res := tf.FormatSelectionSet(s)

	assert.Contains(t, res, "XXX")

	tf.WithNewLine("YYY")

	res = tf.FormatSelectionSet(s)

	assert.Contains(t, res, "YYY")

	newTf := tf.Copy()

	newTf.WithIndent("")

	assert.NotEqual(t, tf.indent, newTf.indent)
}

func TestFormatSelectionSet(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	expected := []string{`{ node { id } }`, `mutation { node { id } }`, `subscription { node { id } }`}
	opTypes := []ast.Operation{ast.Query, ast.Mutation, ast.Subscription}

	for i, exp := range expected {
		res := testFormatter.Copy().WithOperationType(opTypes[i]).FormatSelectionSet(s)

		assert.Equal(t, exp, res)
	}
}

func TestFormatSelectionSetWithOperationName(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	opName := "getNode"
	res := testFormatter.Copy().WithOperationName(opName).FormatSelectionSet(s)

	assert.Equal(t, "query getNode { node { id } }", res)
}

func TestFormatSelectionSetWithArgs(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: "id",
						Type: ast.NamedType("ID!", nil),
					},
				},
			},
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: common.IDFieldName,
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  common.IDFieldName,
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	res := testFormatter.FormatSelectionSet(s)

	assert.Equal(t, `query ($id: ID!) { node(id: $id) { id } }`, res)
}

func TestFormatSelectionSetWithArgsArray(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: "ids",
						Type: ast.NonNullListType(ast.NonNullNamedType("ID", nil), nil),
					},
				},
			},
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: "ids",
					Value: &ast.Value{
						Kind: ast.ListValue,
						Raw:  "",
						Children: ast.ChildValueList{{
							Name: "",
							Value: &ast.Value{
								Kind: ast.Variable,
								Raw:  "id",
								Definition: &ast.Definition{
									Name: "ID",
								},
								ExpectedType: ast.NamedType("ID!", nil),
								Children:     nil,
							},
						}},
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	schema := &ast.Schema{
		Types: map[string]*ast.Definition{
			"ID": {
				Name: "ID",
			},
		},
	}

	res := testFormatter.Copy().WithSchema(schema).FormatSelectionSet(s)

	assert.Equal(t, `query ($id: ID!) { node(ids: [$id]) { id } }`, res)
}

func TestFormatSelectionSetChildrenArgumentList(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: "filterOptions",
						Type: &ast.Type{
							NamedType: "FilterOptions",
							NonNull:   true,
						},
					},
				},
			},
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: "filterOptions",
					Value: &ast.Value{
						Kind: ast.ObjectValue,
						Raw:  "",
						Children: ast.ChildValueList{{
							Name: "nested",
							Value: &ast.Value{
								Kind: ast.ObjectValue,
								Raw:  "",
								Definition: &ast.Definition{
									Name: "Nested",
								},
								Children: ast.ChildValueList{{
									Name: common.IDFieldName,
									Value: &ast.Value{
										Kind: ast.Variable,
										Raw:  common.IDFieldName,
										Definition: &ast.Definition{
											Name: common.IDFieldName,
										},
									},
								}},
							},
						}},
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	schema := &ast.Schema{
		Types: map[string]*ast.Definition{
			"FilterOptions": {
				Name: "FilterOptions",
				Fields: ast.FieldList{{
					Name: "nested",
					Type: ast.NonNullNamedType("Nested", nil),
				}},
			},
			"Nested": {
				Name: "Nested",
				Fields: ast.FieldList{{
					Name: common.IDFieldName,
					Type: ast.NonNullNamedType("ID", nil),
				}},
			},
		},
	}

	res := testFormatter.Copy().WithSchema(schema).FormatSelectionSet(s)

	assert.Equal(t, `query ($id: ID!) { node(filterOptions: {nested:{id:$id}}) { id } }`, res)
}

func TestFormatSelectionSetWithArgsWithOperationName(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: "id",
						Type: ast.NamedType("ID!", nil),
					},
				},
			},
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: common.IDFieldName,
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  common.IDFieldName,
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	opName := "getNode"
	res := testFormatter.Copy().WithOperationName(opName).FormatSelectionSet(s)

	assert.Equal(t, "query getNode($id: ID!) { node(id: $id) { id } }", res)
}

func TestFormatSelectionSetComplex(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: "id",
						Type: ast.NamedType("ID!", nil),
					},
				},
			},
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: common.IDFieldName,
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  common.IDFieldName,
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "test",
							Definition: &ast.FieldDefinition{
								Name: "Test",
								Type: ast.NamedType("Test", nil),
								Arguments: ast.ArgumentDefinitionList{
									&ast.ArgumentDefinition{
										Name: "test",
										Type: ast.NamedType("String!", nil),
									},
								},
							},
							SelectionSet: ast.SelectionSet{
								&ast.Field{
									Name: "test",
								},
							},
							Arguments: ast.ArgumentList{
								&ast.Argument{
									Name: "test",
									Value: &ast.Value{
										Kind: ast.Variable,
										Raw:  "test",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	res := testFormatter.FormatSelectionSet(s)

	assert.Equal(t, `query ($id: ID!, $test: String!) { node(id: $id) { id { test(test: $test) { test } } } }`, res)
}

func TestFormatSelectionSetPersistentOrder(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Name: "Node",
				Type: ast.NamedType("Node", nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: "id1",
						Type: ast.NamedType("ID!", nil),
					},
					&ast.ArgumentDefinition{
						Name: "id2",
						Type: ast.NamedType("ID!", nil),
					},
					&ast.ArgumentDefinition{
						Name: "id3",
						Type: ast.NamedType("ID!", nil),
					},
				},
			},
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: "id1",
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  "id1",
					},
				},
				&ast.Argument{
					Name: "id2",
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  "id2",
					},
				},
				&ast.Argument{
					Name: "id3",
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  "id3",
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	for i := 0; i < 50; i++ {
		res := testFormatter.FormatSelectionSet(s)
		assert.Equal(t, `query ($id1: ID!, $id2: ID!, $id3: ID!) { node(id1: $id1, id2: $id2, id3: $id3) { id } }`, res)
	}

}

func TestFormatSelectionSetInline(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Arguments: ast.ArgumentList{
				&ast.Argument{
					Name: common.IDFieldName,
					Value: &ast.Value{
						Kind: ast.StringValue,
						Raw:  common.IDFieldName,
					},
				},
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
				},
			},
		},
	}

	res := testFormatter.FormatSelectionSet(s)

	assert.Equal(t, `{ node(id: "id") { id } }`, res)
}

func TestFormatComplex(t *testing.T) {
	s := ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: common.IDFieldName,
					Directives: ast.DirectiveList{
						&ast.Directive{
							Name: "dir",
						},
					},
				},
				&ast.InlineFragment{
					TypeCondition: "Test",
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name:  "field",
							Alias: "other",
						},
					},
				},
				&ast.FragmentSpread{
					Name: "Another",
					Definition: &ast.FragmentDefinition{
						Name:          "X",
						TypeCondition: "Another",
						SelectionSet: ast.SelectionSet{
							&ast.Field{
								Name: "field",
							},
						},
					},
				},
			},
		},
	}

	res := testFormatter.FormatSelectionSet(s)

	assert.Equal(t, `{ node { id @dir ... on Test { other: field } ... Another { field } } }`, res)
}
