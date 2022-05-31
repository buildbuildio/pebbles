package executor

import (
	"testing"

	"github.com/buildbuildio/pebbles/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseResponseEmpty(t *testing.T) {
	de := &DepthExecutor{}

	type Case struct {
		ParentType string
		Response   map[string]interface{}
		Result     map[string]interface{}
		IsErr      bool
	}

	for _, c := range []Case{{
		ParentType: "Query",
		Response:   map[string]interface{}{},
		Result:     map[string]interface{}{},
		IsErr:      false,
	}, {
		ParentType: "Query",
		Response:   map[string]interface{}{"id": "1"},
		Result:     map[string]interface{}{"id": "1"},
		IsErr:      false,
	}, {
		ParentType: "Other",
		Response:   map[string]interface{}{"node": map[string]interface{}{"id": "1"}},
		Result:     map[string]interface{}{"id": "1"},
		IsErr:      false,
	}, {
		ParentType: "Other",
		Response:   map[string]interface{}{"node": nil},
		Result:     map[string]interface{}{},
		IsErr:      false,
	}, {
		ParentType: "Other",
		Response:   map[string]interface{}{"id": "1"},
		IsErr:      true,
	}, {
		ParentType: "Other",
		Response:   map[string]interface{}{"node": "1"},
		IsErr:      true,
	}} {
		resps := []*queryerResponse{{
			ExecutionRequest: &ExecutionRequest{
				QueryPlanStep: &planner.QueryPlanStep{
					ParentType: c.ParentType,
				},
				InsertionPoint: nil,
			},
			Response: c.Response,
		}}

		actual, err := de.parseRespones(resps)
		if c.IsErr {
			assert.Error(t, err)
			continue
		}

		require.NoError(t, err)

		assert.EqualValues(t, DepthExecutorResponse{
			ExecutionResults: []*ExecutionResult{{
				InsertionPoint: nil,
				Result:         c.Result,
			}},
			NextExecutionRequests: nil,
		}, *actual)
	}
}
