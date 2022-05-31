package introspection

import (
	"encoding/json"
	"testing"

	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// MockSuccessQueryer responds with pre-defined value when executing a query
type MockSuccessQueryer struct {
	Value string
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
	var res map[string]interface{}
	if err := json.Unmarshal([]byte(q.Value), &res); err != nil {
		return nil, err
	}
	return []map[string]interface{}{res}, nil
}

func (q *MockSuccessQueryer) AsFactory() func(url string) queryer.Queryer {
	return func(url string) queryer.Queryer {
		return q
	}
}

func checkRemoteIntrospectSuccess(t *testing.T, introspectionResult, expected string) {
	q := &MockSuccessQueryer{
		Value: introspectionResult,
	}
	i := ParallelRemoteSchemaIntrospector{Factory: q.AsFactory()}
	schemas, err := i.IntrospectRemoteSchemas("")
	assert.NoError(t, err)
	actual := formatSchema(schemas[0])

	assert.Equal(t, loadAndFormatSchema(expected), actual)
}

func loadAndFormatSchema(input string) string {
	return formatSchema(gqlparser.MustLoadSchema(&ast.Source{Name: "schema", Input: input}))
}
