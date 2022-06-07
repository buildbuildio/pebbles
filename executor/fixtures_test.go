package executor

import (
	"encoding/json"
	"testing"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vektah/gqlparser/v2/ast"
)

// MockSuccessQueryer responds with pre-defined value when executing a query
type MockSuccessQueryer struct {
	Value map[string]interface{}
	url   string
}

var _ queryer.Queryer = &MockSuccessQueryer{}

func (q *MockSuccessQueryer) URL() string {
	if q.url != "" {
		return q.url
	}
	return "mockSuccessQueryer"
}

// Query looks up the name of the query in the map of responses and returns the value
func (q *MockSuccessQueryer) Query(inputs []*requests.Request) ([]map[string]interface{}, error) {
	return []map[string]interface{}{q.Value}, nil
}

// MockQueryerFunc responds to the query by calling the provided function
type MockQueryerFunc struct {
	F   func([]*requests.Request) ([]map[string]interface{}, error)
	url string
}

var _ queryer.Queryer = MockQueryerFunc{}

// Query looks up the name of the query in the map of responses and returns the value
func (q MockQueryerFunc) Query(inputs []*requests.Request) ([]map[string]interface{}, error) {
	return q.F(inputs)
}

// Query looks up the name of the query in the map of responses and returns the value
func (q MockQueryerFunc) URL() string {
	if q.url != "" {
		return q.url
	}
	return "MockQueryerFunc"
}

func selectionSetWithNodeDef(s ast.SelectionSet) ast.SelectionSet {
	return ast.SelectionSet{
		&ast.Field{
			Name: common.NodeFieldName,
			Definition: &ast.FieldDefinition{
				Type: ast.NamedType(common.NodeFieldName, nil),
				Arguments: ast.ArgumentDefinitionList{
					&ast.ArgumentDefinition{
						Name: common.IDFieldName,
						Type: ast.NamedType("ID!", nil),
					},
				},
			},
			Arguments: ast.ArgumentList{
				{
					Name: common.IDFieldName,
					Value: &ast.Value{
						Kind: ast.Variable,
						Raw:  common.IDFieldName,
					},
				},
			},
			SelectionSet: s,
		},
	}
}

func mustCheckEqual(t *testing.T, ctx *ExecutionContext, expected string) {
	pc := &planner.PlanningContext{
		Operation: &ast.OperationDefinition{
			Name:      *ctx.Request.OperationName,
			Operation: ast.Query,
		},
	}
	ctx.QueryPlan = ctx.QueryPlan.SetComputedValues(pc)
	result, err := parallelExecutor.Execute(ctx)
	require.NoError(t, err, "Encountered error executing plan")

	b, err := json.Marshal(result)
	require.NoError(t, err, "Encountered error marshalling response")

	assert.JSONEq(t, expected, string(b))
}
