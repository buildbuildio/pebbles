package executor

import (
	"errors"

	"github.com/buildbuildio/pebbles/common"
)

// ExecutionResult contains result of DepthExecutor executing single ExecutionRequest
type ExecutionResult struct {
	InsertionPoint []string
	Result         map[string]interface{}
}

// DepthExecutorResponse represents single response of DepthExecutor.Execute, containing all accumulated data
type DepthExecutorResponse struct {
	ExecutionResults      []*ExecutionResult
	NextExecutionRequests []*ExecutionRequest
}

func (de *DepthExecutor) parseRespones(queryerResponses []*queryerResponse) (*DepthExecutorResponse, error) {
	res, err := common.AsyncMapReduce(
		queryerResponses,
		new(DepthExecutorResponse),
		func(field *queryerResponse) (*DepthExecutorResponse, error) {
			queryResult := field.Response
			req := field.ExecutionRequest
			step := req.QueryPlanStep

			// NOTE: this insertion point could point to a list of values. If it did, we have to have
			//       passed it to the this invocation of this function. It is safe to trust this
			//       InsertionPoint as the right place to insert this result.

			// if this is a query that falls underneath a `node(id: ???)` query then we only want to consider the object
			// underneath the `node` field as the result for the query
			if !common.IsRootObjectName(step.ParentType) {
				// get the result from the response that we have to stitch there
				qr, ok := queryResult[common.NodeFieldName]
				if !ok {
					return nil, req.ToGqlError(errors.New("missing node key when expected"))
				}

				// if node returned nil, we expect queryResult to be empty map
				if qr == nil {
					queryResult = make(map[string]interface{})
				} else {
					qrMap, ok := qr.(map[string]interface{})
					if !ok {
						return nil, req.ToGqlError(errors.New("node is not a map"))
					}

					queryResult = qrMap
				}
			}

			// if there are next steps
			nextExecutionRequests, err := de.findNextExecutionRequests(req.InsertionPoint, step, queryResult)
			if err != nil {
				return nil, req.ToGqlError(err)
			}

			return &DepthExecutorResponse{
				NextExecutionRequests: nextExecutionRequests,
				ExecutionResults: []*ExecutionResult{{
					InsertionPoint: req.InsertionPoint,
					Result:         queryResult,
				}},
			}, nil
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
