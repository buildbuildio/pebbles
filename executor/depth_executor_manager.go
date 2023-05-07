package executor

import (
	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/planner"
)

type DepthExecutorManager struct {
	ctx                   *ExecutionContext
	depthExecutors        map[int]*DepthExecutor
	pointDataExtractor    PointDataExtractor
	maxDepth              int
	result                map[string]interface{}
	initialInsertionPoint []string
}

func NewDepthExecutorManager(ctx *ExecutionContext) *DepthExecutorManager {
	queryPlanSteps := make(map[int][]*planner.QueryPlanStep)

	for _, qp := range ctx.QueryPlan.RootSteps {
		walkPlanStep(qp, 0, queryPlanSteps)
	}

	depthExecutors := make(map[int]*DepthExecutor)
	pointDataExtractor := &CachedPointDataExtractor{cache: make(map[string]*PointData)}

	var maxDepth int
	for depth, qps := range queryPlanSteps {
		depthExecutors[depth] = &DepthExecutor{
			QueryPlanSteps:     qps,
			Depth:              depth,
			PointDataExtractor: pointDataExtractor,
			ctx:                ctx,
		}
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	result := make(map[string]interface{})
	if ctx.InitialResult != nil {
		result = ctx.InitialResult
	}

	return &DepthExecutorManager{
		ctx:                ctx,
		depthExecutors:     depthExecutors,
		pointDataExtractor: pointDataExtractor,
		maxDepth:           maxDepth,
		result:             result,
	}
}

// Execute takes ExecutionContext and performs sequential execution for each depth in the plan.
func (dem *DepthExecutorManager) Execute() (map[string]interface{}, error) {
	executionRequests := make([]*ExecutionRequest, 0)
	errs := gqlerrors.ErrorList{}

	// for initial step construct root queries
	for _, step := range dem.depthExecutors[0].QueryPlanSteps {
		insertionPoint := []string{}
		if step.InsertionPoint != nil {
			insertionPoint = step.InsertionPoint
		}

		executionRequests = append(executionRequests, &ExecutionRequest{
			QueryPlanStep:  step,
			InsertionPoint: insertionPoint,
		})
	}

	for depth := 0; depth <= dem.maxDepth; depth++ {
		if len(executionRequests) == 0 {
			break
		}

		de := dem.depthExecutors[depth]

		// execute each request async
		// TODO: propagate all errors if possible
		exResp, executionErr := de.Execute(executionRequests)
		if executionErr != nil {
			errs = gqlerrors.ExtendErrorList(errs, executionErr)
			return nil, errs
		}

		// merge into acc body
		if err := dem.merge(exResp); err != nil {
			return nil, err
		}
		// set next execution requests, obtained from current depth
		executionRequests = exResp.NextExecutionRequests
	}

	return dem.result, nil
}

func (dem *DepthExecutorManager) merge(resp *DepthExecutorResponse) error {
	for _, res := range resp.ExecutionResults {
		// this will triggers when we reach depth > 1, because at this point dem.Result will not be emty
		// so we need to find exact place where to insert our execution result
		if len(res.InsertionPoint) > 0 {
			// a pointer to the objects we are modifying
			// note, that here we indeed want to modify initial structure of dem.Result, so merging maps later will work
			// it's safe to do here, as Merge not called in parallel
			targetObj, err := ExtractValueModifyingSource(dem.pointDataExtractor, dem.result, res.InsertionPoint)
			if err != nil {
				return err
			}

			// if the value we are assigning is an object
			for k, v := range res.Result {
				v1, ok1 := v.(map[string]interface{})
				v2, ok2 := targetObj[k].(map[string]interface{})
				if ok1 && ok2 {
					targetObj[k] = mergeMaps(v2, v1)
				} else {
					targetObj[k] = v
				}
			}
		} else {
			// depth = 1, no dem.Result yet
			for key, value := range res.Result {
				v1, ok1 := value.(map[string]interface{})
				v2, ok2 := dem.result[key].(map[string]interface{})
				if ok1 && ok2 {
					dem.result[key] = mergeMaps(v2, v1)
				} else {
					dem.result[key] = value
				}
			}
		}
	}

	return nil
}

// walkPlanStep goes recursively over qps generating map[depth][]*planner.QueryPlanStep and setting queriers inplace along the way for each QueryPlanStep.
func walkPlanStep(
	qps *planner.QueryPlanStep,
	depth int,
	acc map[int][]*planner.QueryPlanStep,
) {
	acc[depth] = append(acc[depth], qps)

	if qps.Then == nil {
		return
	}

	for _, nps := range qps.Then {
		// go recursively for next depth
		walkPlanStep(nps, depth+1, acc)
	}
}
