package executor

import (
	"strconv"
	"testing"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/stretchr/testify/require"
)

func TestExecuteRequestsSameNodeIDs(t *testing.T) {
	pointDataExtractor := &CachedPointDataExtractor{cache: make(map[string]*PointData)}

	de := DepthExecutor{
		ctx: &ExecutionContext{
			Queryers: map[string]queryer.Queryer{
				"0": MockQueryerFunc{
					F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
						t.Helper()
						var res []map[string]interface{}
						require.Len(t, inputs, 3)
						for _, input := range inputs {
							v := input.Variables[common.IDFieldName]
							res = append(res, map[string]interface{}{
								common.IDFieldName: v,
							})
						}

						return res, nil
					},
				},
			},
		},
		PointDataExtractor: pointDataExtractor,
	}

	createExecutionRequest := func(i int) *ExecutionRequest {
		t.Helper()
		return &ExecutionRequest{
			QueryPlanStep: &planner.QueryPlanStep{
				URL:           "0",
				VariablesList: nil,
			},
			InsertionPoint: []string{"test", "user#" + strconv.Itoa(i)},
		}
	}

	for i, order := range [][]int{
		{1, 2, 3, 1, 2},
		{1, 2, 2, 2, 3},
		{2, 1, 1, 1, 3},
		{2, 2, 2, 1, 3},
		{1, 2, 3, 3, 3},
		{3, 3, 3, 2, 1},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var ers []*ExecutionRequest
			for _, ind := range order {
				ers = append(ers, createExecutionRequest(ind))
			}

			resp, err := de.executeRequests(ers)
			require.NoError(t, err)
			require.Len(t, resp, 5)

			for i, ind := range order {
				r := resp[i]
				require.EqualValues(t, strconv.Itoa(ind), r.Response[common.IDFieldName])
			}
		})
	}

}
