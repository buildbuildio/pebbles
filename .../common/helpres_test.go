package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestIsEqual(t *testing.T) {
	type Case struct {
		A      []string
		B      []string
		Result bool
	}

	for _, c := range []Case{{
		A:      []string{"1", "2"},
		B:      []string{"1", "2"},
		Result: true,
	}, {
		A:      []string{"1"},
		B:      []string{"1", "2"},
		Result: false,
	}, {
		A:      []string{"1", "2"},
		B:      []string{"1"},
		Result: false,
	}, {
		A:      []string{"1", "2"},
		B:      []string{"1", "3"},
		Result: false,
	}} {
		assert.Equal(t, c.Result, IsEqual(c.A, c.B))
	}
}

func TestSelectionSetToFields(t *testing.T) {
	ss := ast.SelectionSet{
		&ast.Field{
			Name: "Test",
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: "Another",
				},
			},
		},
		&ast.InlineFragment{SelectionSet: ast.SelectionSet{&ast.Field{
			Name: "Other",
		}}},
	}

	fields := SelectionSetToFields(ss, nil)

	assert.Len(t, fields, 2)

	assert.Equal(t, "Test", fields[0].Name)
	assert.Equal(t, "Other", fields[1].Name)
}

func TestSelectionSetToFieldsWithParent(t *testing.T) {
	def := &ast.Definition{
		Fields: ast.FieldList{&ast.FieldDefinition{
			Name: "Test",
		}},
	}
	ss := ast.SelectionSet{
		&ast.Field{
			Name: "Test",
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: "Another",
				},
			},
		},
		&ast.Field{
			Name: "AnotherTest",
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: "Another",
				},
			},
		},
	}

	fields := SelectionSetToFields(ss, def)

	require.Len(t, fields, 1)

	assert.Equal(t, "Test", fields[0].Name)
}

func TestSelectionSetToFieldsWithParentInlineFragment(t *testing.T) {
	def := &ast.Definition{
		Name: "Test",
		Fields: ast.FieldList{&ast.FieldDefinition{
			Name: "Test",
		}},
	}
	ss := ast.SelectionSet{
		&ast.InlineFragment{
			TypeCondition: "Test", SelectionSet: ast.SelectionSet{&ast.Field{Name: "Test"}},
		},
		&ast.InlineFragment{
			TypeCondition: "Another", SelectionSet: ast.SelectionSet{&ast.Field{Name: "Another"}},
		},
	}

	fields := SelectionSetToFields(ss, def)

	require.Len(t, fields, 1)

	assert.Equal(t, "Test", fields[0].Name)
}

func TestAsyncMapSuccess(t *testing.T) {
	payload := []int{1, 2, 3, 4, 5}
	var acc int
	actual, err := AsyncMapReduce(payload, acc, func(field int) (int, error) { return field, nil }, func(acc int, value int) int { return acc + value })
	assert.Nil(t, err)
	assert.Equal(t, 15, actual)
}

func TestAsyncMapError(t *testing.T) {
	payload := []int{1, 2, 3, 4, 5}
	var acc int
	_, err := AsyncMapReduce(payload, acc, func(field int) (int, error) { return 0, errors.New("error") }, func(acc int, value int) int { return acc + value })
	assert.Error(t, err)
}
