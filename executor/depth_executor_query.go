package executor

import (
	"errors"
	"fmt"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/requests"
)

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
		Path:       er.InsertionPoint,
		Extensions: nil,
		Message:    err.Error(),
	}
}

type queryerBatchRequest struct {
	Inputs            []*requests.Request
	ExecutionRequests []*ExecutionRequest
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
	batchRequest := &queryerBatchRequest{
		Inputs:            make([]*requests.Request, len(ers)),
		ExecutionRequests: make([]*ExecutionRequest, len(ers)),
	}

	for i, req := range ers {
		variables, err := de.getVariables(req)
		if err != nil {
			return nil, err
		}
		// form input
		input := &requests.Request{
			Query:         req.QueryPlanStep.QueryString,
			Variables:     variables,
			OperationName: req.QueryPlanStep.OperationName,
		}
		batchRequest.Inputs[i] = input
		batchRequest.ExecutionRequests[i] = req
	}

	q, ok := de.ctx.Queryers[ers[0].QueryPlanStep.URL]
	if !ok {
		return nil, fmt.Errorf("unable to find queryer for: %s", ers[0].QueryPlanStep.URL)
	}

	resps, err := q.Query(batchRequest.Inputs)
	if err != nil {
		return nil, err
	}

	if len(resps) != len(batchRequest.Inputs) {
		return nil, errors.New("not all requests were fetched")
	}

	qResps := make([]*queryerResponse, len(batchRequest.Inputs))
	for i, resp := range resps {
		qResps[i] = &queryerResponse{
			Response:         resp,
			ExecutionRequest: batchRequest.ExecutionRequests[i],
		}
	}

	return qResps, nil
}
