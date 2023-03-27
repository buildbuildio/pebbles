package executor

import (
	"errors"
	"fmt"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/samber/lo"
)

type indexMap map[string][]int

// Set sets indexes for value and returns true if it's a new value
func (im indexMap) Set(index int, value interface{}) bool {
	v, ok := value.(string)
	if !ok {
		return true
	}
	if im[v] == nil {
		im[v] = make([]int, 0)
	}

	im[v] = append(im[v], index)
	return len(im[v]) == 1
}

// GetSameIndexes returns all indexes associated with provided.
// If it's the ony one it return null
// Otherwise all indexes included provided are returned
func (im indexMap) GetSameIndexes(i int) []int {
	for _, v := range im {
		if v[0] == i {
			if len(v) != 1 {
				return v
			}
			break
		}
	}
	return nil
}

// ExecutionRequest contains all data needed for DepthExecutor to execute request
type ExecutionRequest struct {
	QueryPlanStep  *planner.QueryPlanStep
	InsertionPoint []string
}

// ToGqlError takes error and produces *gqlerrors.Error using er.InsertionPoint as path and err.Error as message.
// If err is already *gqlerrors.Error, it returns it as is.
func (er ExecutionRequest) ToGqlError(err error) *gqlerrors.Error {
	if e, ok := err.(*gqlerrors.Error); ok {
		return e
	}

	return &gqlerrors.Error{
		Path:       lo.Map(er.InsertionPoint, func(el string, i int) interface{} { return el }),
		Extensions: nil,
		Message:    err.Error(),
	}
}

type queryerResponse struct {
	ExecutionRequest *ExecutionRequest
	Response         map[string]interface{}
}

// getVariables determines which variables we need to send with provided request
func (de *DepthExecutor) getVariables(req *ExecutionRequest) (map[string]interface{}, error) {
	// the list of variables and their definitions that pertain to this query
	variables := map[string]interface{}{}

	// we need to grab the variable definitions and values for each variable in the step
	for _, variable := range req.QueryPlanStep.VariablesList {
		if de.ctx.Request.Variables == nil {
			break
		}
		// and the value if it exists
		if value, ok := de.ctx.Request.Variables[variable]; ok {
			variables[variable] = value
		}
	}

	// the id of the object we are query is defined by the last step in the realized insertion point
	if len(req.InsertionPoint) > 0 {
		head := req.InsertionPoint[len(req.InsertionPoint)-1]

		// get the data of the point
		pointData, err := de.PointDataExtractor.Extract(head)
		if err != nil {
			return nil, err
		}

		// if we dont have an id
		if pointData.ID == "" {
			return nil, errors.New("could not find id in path")
		}

		// save the id as a variable to the query
		variables[common.IDFieldName] = pointData.ID
	}

	return variables, nil
}

// prepareRequests walks through ers, appending information about the query (params and operation name)
func (de *DepthExecutor) executeRequests(ers []*ExecutionRequest) ([]*queryerResponse, error) {
	if len(ers) == 0 {
		return nil, nil
	}

	// all requests already grouped by queryer
	batchRequest := make([]*requests.Request, 0, len(ers))
	iMap := make(indexMap)

	for i, req := range ers {
		variables, err := de.getVariables(req)
		if err != nil {
			return nil, err
		}
		// exclude same requests to optimize queryer
		// check insertion point, if it's not empty, then it's a child query
		// which is always node
		if len(req.InsertionPoint) > 0 {
			if id, ok := variables[common.IDFieldName]; ok {
				isNewValue := iMap.Set(i, id)
				if !isNewValue {
					continue
				}
			}
		}

		// form input
		input := &requests.Request{
			Query:         req.QueryPlanStep.QueryString,
			Variables:     variables,
			OperationName: req.QueryPlanStep.OperationName,
		}
		batchRequest = append(batchRequest, input)
	}

	q, ok := de.ctx.Queryers[ers[0].QueryPlanStep.URL]
	if !ok {
		return nil, fmt.Errorf("unable to find queryer for: %s", ers[0].QueryPlanStep.URL)
	}

	resps, err := q.Query(batchRequest)
	if err != nil {
		return nil, err
	}

	if len(resps) != len(batchRequest) {
		return nil, errors.New("not all requests were fetched")
	}

	qResps := make([]*queryerResponse, len(ers))
	for i, resp := range resps {
		indexes := iMap.GetSameIndexes(i)
		if indexes == nil {
			qResps[i] = &queryerResponse{
				Response:         resp,
				ExecutionRequest: ers[i],
			}
			continue
		}

		for _, ind := range indexes {
			var copyResp map[string]interface{}
			copyResp, err := copyMap(resp)
			if err != nil {
				return nil, err
			}
			qResps[ind] = &queryerResponse{
				Response:         copyResp,
				ExecutionRequest: ers[ind],
			}
		}
	}

	return qResps, nil
}
