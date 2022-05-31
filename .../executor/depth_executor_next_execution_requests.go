package executor

import (
	"strings"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/planner"
)

// findNextExecutionRequests inspects queryResult of current step and decides which requests need to be executed next on metadata inside step.Then.
func (de *DepthExecutor) findNextExecutionRequests(
	insertionPoint []string,
	step *planner.QueryPlanStep,
	queryResult map[string]interface{},
) ([]*ExecutionRequest, error) {
	if len(step.Then) == 0 {
		return nil, nil
	}

	// we need to find the ids of the objects we are inserting into and then kick of the worker with the right
	// insertion point. For lists, insertion points look like: ["user", "friends:0", "catPhotos:0", "owner"]

	// if copiedInsertionPoint is empty it means, that it's a root query with no ids, so we can cache the results
	// f.e. ["user": "friends"]
	if len(insertionPoint) == 0 {
		return findNextExecutionRequestsWithCache(insertionPoint, step, queryResult)
	}
	// otherwise it's better to get them async
	return findNextExecutionRequestsAsync(insertionPoint, step, queryResult)
}

// findNextExecutionRequestsWithCache go over step.Then sequently and cache results
// it only triggers on depth = 1, there're usually huge body with same insertion points for each dependant
func findNextExecutionRequestsWithCache(
	insertionPoint []string,
	step *planner.QueryPlanStep,
	queryResult map[string]interface{},
) ([]*ExecutionRequest, error) {
	var insertPoints [][]string
	var err error
	var nextExecutionRequests []*ExecutionRequest
	executorFindInsertionPointsCache := make(map[string][][]string)
	for _, dependent := range step.Then {
		// joining on symbol we will not find in insertion point
		key := strings.Join(dependent.InsertionPoint, "❤️")
		v, ok := executorFindInsertionPointsCache[key]
		if !ok {
			insertPoints, err = FindInsertionPoints(
				dependent.InsertionPoint,
				step.SelectionSet,
				queryResult,
				[][]string{insertionPoint},
			)
			if err != nil {
				return nil, err
			}

			executorFindInsertionPointsCache[key] = copy2DStringArray(insertPoints)
		} else {
			insertPoints = copy2DStringArray(v)
		}

		for _, ip := range insertPoints {
			nextExecutionRequests = append(nextExecutionRequests, &ExecutionRequest{
				QueryPlanStep:  dependent,
				InsertionPoint: ip,
			})
		}
	}

	return nextExecutionRequests, nil
}

func findNextExecutionRequestsAsync(
	insertionPoint []string,
	step *planner.QueryPlanStep,
	queryResult map[string]interface{},
) ([]*ExecutionRequest, error) {
	var nextExecutionRequests []*ExecutionRequest

	res, err := common.AsyncMapReduce(
		step.Then,
		nextExecutionRequests,
		func(field *planner.QueryPlanStep) ([]*ExecutionRequest, error) {
			insertPoints, err := FindInsertionPoints(
				field.InsertionPoint,
				step.SelectionSet,
				queryResult,
				[][]string{insertionPoint},
			)
			if err != nil {
				return nil, err
			}
			var reqs []*ExecutionRequest
			for _, ip := range insertPoints {
				reqs = append(reqs, &ExecutionRequest{
					QueryPlanStep:  field,
					InsertionPoint: ip,
				})
			}
			return reqs, nil
		},
		func(acc []*ExecutionRequest, value []*ExecutionRequest) []*ExecutionRequest {
			return append(acc, value...)
		},
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}
