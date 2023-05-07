package pebbles

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/executor"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/playground"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/exp/slices"
)

type MockRemoteSchemaIntrospector struct {
	Res []*ast.Schema
}

func (mi *MockRemoteSchemaIntrospector) IntrospectRemoteSchemas(urls ...string) ([]*ast.Schema, error) {
	return mi.Res, nil
}

type MockPlanner struct {
	Res   *planner.QueryPlan
	Error error
}

func (mp *MockPlanner) Plan(ctx *planner.PlanningContext) (*planner.QueryPlan, error) {
	return mp.Res, mp.Error
}

type MockExecutor struct {
	Res   map[string]interface{}
	Error error
}

func (me *MockExecutor) Execute(*executor.ExecutionContext) (map[string]interface{}, error) {
	return me.Res, me.Error
}

func TestGatewayGetQueryers(t *testing.T) {
	factory := func(pc *planner.PlanningContext, s string) queryer.Queryer {
		return nil
	}
	gw := &Gateway{
		queryerFactory: factory,
	}

	ps := []*planner.QueryPlanStep{{
		URL: "1",
		Then: []*planner.QueryPlanStep{{
			URL: "2",
		}, {
			URL: "3",
		}},
	}, {
		URL: "2",
	}, {
		URL: "2",
		Then: []*planner.QueryPlanStep{{
			URL: "4",
		}},
	}}

	queryers := gw.getQueryers(nil, ps)

	keys := lo.Keys(queryers)
	slices.Sort(keys)
	assert.EqualValues(t, []string{"1", "2", "3", "4"}, keys)
}

func TestGatewayMissingQueryError(t *testing.T) {
	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway([]string{""}, WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`{}`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res map[string]interface{}
	json.Unmarshal(b, &res)

	assert.Equal(t, "missing query from request", res["errors"].([]interface{})[0].(map[string]interface{})["message"])
}

func TestGatewayWrongQueryError(t *testing.T) {
	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway([]string{""}, WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`{"operationName": "test", "query": "{ otherTest }", "variables": null}`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res map[string]interface{}
	json.Unmarshal(b, &res)

	assert.Equal(t, "Cannot query field \"otherTest\" on type \"Query\".", res["errors"].([]interface{})[0].(map[string]interface{})["message"])
}

func TestGatewayPlannerError(t *testing.T) {
	mp := &MockPlanner{
		Error: errors.New("planner"),
	}
	me := &MockExecutor{
		Res: map[string]interface{}{
			"test": "YES",
		},
	}

	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway([]string{""}, WithExecutor(me), WithPlanner(mp), WithDefaultPlayground(), WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`{"operationName": "test", "query": "query test { test }", "variables": null}`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res map[string]interface{}
	json.Unmarshal(b, &res)

	assert.Equal(t, "planner", res["errors"].([]interface{})[0].(map[string]interface{})["message"])
}

func TestGatewayExecutorError(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{},
	}
	me := &MockExecutor{
		Error: errors.New("executor"),
	}

	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway([]string{""}, WithExecutor(me), WithPlanner(mp), WithDefaultPlayground(), WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`{"operationName": "test", "query": "query test { test }", "variables": null}`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res map[string]interface{}
	json.Unmarshal(b, &res)

	assert.Equal(t, "executor", res["errors"].([]interface{})[0].(map[string]interface{})["message"])
}

func TestGatewaySingleQuery(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				URL:        "0",
				ParentType: "Query",
				SelectionSet: ast.SelectionSet{&ast.Field{
					Name:         "test",
					SelectionSet: nil,
				}},
				InsertionPoint: nil,
				Then:           nil,
			}},
		},
	}
	me := &MockExecutor{
		Res: map[string]interface{}{
			"test": "YES",
		},
	}

	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway(
		[]string{""},
		WithExecutor(me),
		WithPlanner(mp),
		WithDefaultPlayground(),
		WithGetParentTypeFromIDFunc(func(id interface{}) (string, bool) {
			return "", false
		}),
		WithRemoteSchemaIntrospector(mi),
	)
	assert.NoError(t, err)

	for _, payload := range []string{
		`{"operationName": "test", "query": "query test { test }", "variables": null}`,
		`{"operationName": null, "query": "query test { test }", "variables": null}`,
	} {
		buf := &bytes.Buffer{}

		buf.WriteString(payload)

		r, err := http.NewRequest("POST", "localhost", buf)
		assert.NoError(t, err)

		f := http.HandlerFunc(gw.Handler)

		rr := httptest.NewRecorder()

		f(rr, r)

		b := rr.Body.Bytes()

		var res map[string]interface{}
		json.Unmarshal(b, &res)

		assert.Equal(t, "YES", res["data"].(map[string]interface{})["test"])
		_, hasErrorsKeyword := res["errors"]
		assert.False(t, hasErrorsKeyword)
	}
}

func TestGatewaySingleQueryWrongOperationName(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				URL:        "0",
				ParentType: "Query",
				SelectionSet: ast.SelectionSet{&ast.Field{
					Name:         "test",
					SelectionSet: nil,
				}},
				InsertionPoint: nil,
				Then:           nil,
			}},
		},
	}
	me := &MockExecutor{
		Res: map[string]interface{}{
			"test": "YES",
		},
	}

	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway([]string{""}, WithExecutor(me), WithPlanner(mp), WithDefaultPlayground(), WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`{"operationName": "wrong", "query": "query test { test }", "variables": null}`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res map[string]interface{}
	json.Unmarshal(b, &res)

	_, hasErrorsKeyword := res["errors"]
	assert.True(t, hasErrorsKeyword)
}

func TestGatewayMultipleQueries(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{},
	}
	me := &MockExecutor{
		Res: map[string]interface{}{
			"test": "YES",
		},
	}

	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	var p playground.DefaultPlayground
	gw, err := NewGateway([]string{""}, WithExecutor(me), WithPlanner(mp), WithPlaygroundProvider(p), WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`[{"operationName": "test", "query": "query test { test }", "variables": null}, {"operationName": "test", "query": "query test { test }", "variables": null}]`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res []map[string]interface{}
	json.Unmarshal(b, &res)

	assert.Len(t, res, 2)

	assert.Equal(t, "YES", res[0]["data"].(map[string]interface{})["test"])

	for _, r := range res {
		_, hasErrorsKeyword := r["errors"]
		assert.False(t, hasErrorsKeyword)
	}
}

func TestGatewayIntrospectionQuery(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				URL:        common.InternalServiceName,
				ParentType: "Query",
				SelectionSet: ast.SelectionSet{&ast.Field{
					Name: "__schema",
					SelectionSet: ast.SelectionSet{&ast.Field{
						Name: "types",
						SelectionSet: ast.SelectionSet{&ast.Field{
							Name: "name",
						}},
					}},
				}},
				InsertionPoint: nil,
				Then:           nil,
			}},
		},
	}
	me := &MockExecutor{
		Res: map[string]interface{}{
			"test": "YES",
		},
	}

	schema := `
		type Query {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}
	gw, err := NewGateway([]string{""}, WithExecutor(me), WithPlanner(mp), WithRemoteSchemaIntrospector(mi))
	assert.NoError(t, err)

	buf := &bytes.Buffer{}

	buf.WriteString(`{"operationName": "introspectionQuery", "query": "query introspectionQuery { __schema {types {name}} }", "variables": null}`)

	r, err := http.NewRequest("POST", "localhost", buf)
	assert.NoError(t, err)

	f := http.HandlerFunc(gw.Handler)

	rr := httptest.NewRecorder()

	f(rr, r)

	b := rr.Body.Bytes()

	var res map[string]interface{}
	json.Unmarshal(b, &res)

	assert.NotEmpty(t, res["data"].(map[string]interface{}))
}
