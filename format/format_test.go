package format

import (
	"testing"

	"github.com/buildbuildio/pebbles/common"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestFormatWithArgs(t *testing.T) {
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

	res := DebugFormatSelectionSetWithArgs(s)

	assert.Equal(t, `query ($id: ID!) { node(id: $id) { id }}`, res)
}

func TestFormatWithArgsWithOperationName(t *testing.T) {
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
	res := FormatSelectionSetWithArgs(s, &opName)

	assert.Equal(t, "query getNode ($id: ID!) {\n\tnode(id: $id) {\n\t\tid\n\t}\n}", res)
}

func TestFormatWithArgsComplex(t *testing.T) {

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

	res := DebugFormatSelectionSetWithArgs(s)

	assert.Equal(t, `query ($id: ID!, $test: String!) { node(id: $id) { id { test(test: $test) { test } } }}`, res)
}

func TestFormatWithArgsPersistentOrder(t *testing.T) {
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
		res := DebugFormatSelectionSetWithArgs(s)
		assert.Equal(t, `query ($id1: ID!, $id2: ID!, $id3: ID!) { node(id1: $id1, id2: $id2, id3: $id3) { id }}`, res)
	}

}

func TestFormat(t *testing.T) {
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

	res := DebugFormatSelectionSetWithArgs(s)

	assert.Equal(t, `{ node { id }}`, res)
}

func TestFormatWithArgsInline(t *testing.T) {
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

	res := DebugFormatSelectionSetWithArgs(s)

	assert.Equal(t, `{ node(id: "id") { id }}`, res)
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

	res := DebugFormatSelectionSetWithArgs(s)

	assert.Equal(t, `{ node { id @dir ... on Test { other: field } ... Another { field } }}`, res)
}
