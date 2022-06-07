package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestGetVariablesList(t *testing.T) {
	ss := ast.SelectionSet{&ast.Field{
		Name: "test",
		Arguments: ast.ArgumentList{
			&ast.Argument{
				Name: "t1",
				Value: &ast.Value{
					Raw: "a",
				},
			},
			&ast.Argument{
				Name: "t2",
				Value: &ast.Value{
					Raw: "",
					Children: ast.ChildValueList{{
						Name: "t3",
						Value: &ast.Value{
							Raw: "b",
						},
					}, {
						Name: "t4",
						Value: &ast.Value{
							Raw: "",
							Children: ast.ChildValueList{{
								Name: "t5",
								Value: &ast.Value{
									Raw: "c",
								},
							}},
						},
					}},
				},
			},
		},
	}}

	qps := &QueryPlanStep{
		SelectionSet: ss,
	}
	qps.setVariablesList()

	assert.Equal(t, qps.VariablesList, []string{"a", "b", "c"})
}

func TestGetVariablesListEmpty(t *testing.T) {
	ss := ast.SelectionSet{&ast.Field{
		Name: "test",
	}}

	qps := &QueryPlanStep{
		SelectionSet: ss,
	}
	qps.setVariablesList()

	assert.Nil(t, qps.VariablesList)
}
