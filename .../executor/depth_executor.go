package executor

import (
	"errors"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/samber/lo"
)

// DepthExecutor uses provided QueryPlanSteps to execute against finite backend
// and provide requests for for DepthExecutor on next depth
type DepthExecutor struct {
	ctx                *ExecutionContext
	QueryPlanSteps     []*planner.QueryPlanStep
	PointDataExtractor PointDataExtractor
	Depth              int
}

// Execute takes execution requests and runs them async, gathering all errors and results
func (de *DepthExecutor) Execute(ers []*ExecutionRequest) (*DepthExecutorResponse, error) {
	if len(ers) == 0 {
		return nil, errors.New("empty request list")
	}

	// group by requests by corresponding queryer
	groupedRequests := lo.PartitionBy(ers, func(x *ExecutionRequest) string {
		return x.QueryPlanStep.URL
	})

	res, err := common.AsyncMapReduce(
		groupedRequests,
		new(DepthExecutorResponse),
		func(field []*ExecutionRequest) (*DepthExecutorResponse, error) {
			// compute general values like query variables, operationName and etc.
			qResps, err := de.executeRequests(field)
			if err != nil {
				return nil, err
			}

			// parse response
			res, err := de.parseRespones(qResps)
			if err != nil {
				return nil, err
			}

			return res, nil
		},
		func(acc *DepthExecutorResponse, value *DepthExecutorResponse) *DepthExecutorResponse {
			acc.ExecutionResults = append(acc.ExecutionResults, value.ExecutionResults...)
			acc.NextExecutionRequests = append(acc.NextExecutionRequests, value.NextExecutionRequests...)
			return acc
		},
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}
