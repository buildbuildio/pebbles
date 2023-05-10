package pebbles

import (
	"encoding/json"
	"errors"
	"net"
	"sync"

	"github.com/buildbuildio/pebbles/executor"
	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/gobwas/ws/wsutil"
)

type subscriptionEntry struct {
	id           string
	request      *requests.Request
	gateway      *Gateway
	originalPlan *planner.QueryPlan
	isClosed     bool

	closeCh        chan struct{}
	queryerCloseCh chan struct{}
	respCh         chan *requests.Response
	executorFn     func(map[string]interface{}) (map[string]interface{}, error)

	sync.Mutex
}

func (g *Gateway) newSubscriptionEntry(id string, ctx *planner.PlanningContext) (*subscriptionEntry, error) {
	subEntry := &subscriptionEntry{
		id:             id,
		closeCh:        make(chan struct{}),
		queryerCloseCh: make(chan struct{}),
		respCh:         make(chan *requests.Response),
	}

	// get the plan for specific query
	plan, err := g.planner.Plan(ctx)
	if err != nil {
		return nil, err
	}

	subEntry.originalPlan = plan

	additionalRootSteps := make([]*planner.QueryPlanStep, 0)

	for _, rs := range plan.RootSteps {
		additionalRootSteps = append(additionalRootSteps, rs.Then...)
		rs.Then = nil
	}

	rootQueryers := g.getQueryers(ctx, plan.RootSteps)

	if len(plan.RootSteps) != 1 {
		return nil, errors.New("too many root operations")
	}

	rootStep := plan.RootSteps[0]

	queryer := rootQueryers[rootStep.URL]

	rootRequest := &requests.Request{
		Original:      ctx.Request.Original,
		Query:         rootStep.QueryString,
		Variables:     ctx.Request.Variables,
		OperationName: ctx.Request.OperationName,
	}
	if err := queryer.Subscribe(rootRequest, subEntry.queryerCloseCh, subEntry.respCh); err != nil {
		return nil, err
	}

	if len(additionalRootSteps) != 0 {
		additionalQueryers := g.getQueryers(ctx, additionalRootSteps)

		subEntry.executorFn = func(initialResult map[string]interface{}) (map[string]interface{}, error) {
			newRootSteps := make([]*planner.QueryPlanStep, 0)
			for _, step := range additionalRootSteps {
				insertionPoints, err := executor.FindInsertionPoints(
					step.InsertionPoint,
					rootStep.SelectionSet,
					initialResult,
					[][]string{rootStep.InsertionPoint},
				)
				if err != nil {
					return nil, gqlerrors.FormatError(err)
				}
				for _, insertionPoint := range insertionPoints {
					cpy := *step
					cpy.InsertionPoint = insertionPoint
					newRootSteps = append(newRootSteps, &cpy)
				}
			}

			result, err := g.executor.Execute(&executor.ExecutionContext{
				QueryPlan: &planner.QueryPlan{
					RootSteps:   newRootSteps,
					ScrubFields: plan.ScrubFields,
				},
				Request:       ctx.Request,
				Queryers:      additionalQueryers,
				InitialResult: initialResult,
			})

			plan.ScrubFields.Clean(result)
			return result, err
		}
	}

	return subEntry, nil
}

func (se *subscriptionEntry) prepareResponse(resp *requests.Response) *requests.Response {
	// error occured or no subrequests required
	if len(resp.Errors) != 0 || resp.Data == nil || se.executorFn == nil {
		se.originalPlan.ScrubFields.Clean(resp.Data)
		return resp
	}

	fullResp, execErr := se.executorFn(resp.Data)
	return &requests.Response{
		Errors: gqlerrors.FormatError(execErr),
		Data:   fullResp,
	}
}

func (se *subscriptionEntry) Close() {
	se.TryLock()
	isClosed := se.isClosed
	se.Unlock()
	if isClosed {
		return
	}
	se.closeCh <- struct{}{}
}

func (se *subscriptionEntry) Listen(conn net.Conn) {
	defer func() {
		se.queryerCloseCh <- struct{}{}
		se.Lock()
		defer se.Unlock()
		close(se.queryerCloseCh)
		close(se.closeCh)
		close(se.respCh)
		se.isClosed = true
	}()

	for {
		select {
		case resp := <-se.respCh:
			if resp == nil {
				return
			}
			resp = se.prepareResponse(resp)
			bResp, err := json.Marshal(requests.ServerSubMsg{
				ID:      se.id,
				Type:    requests.SubData,
				Payload: resp,
			})
			if err != nil {
				return
			}
			if err := wsutil.WriteServerText(conn, bResp); err != nil {
				return
			}
		case <-se.closeCh:
			return
		}

	}
}
