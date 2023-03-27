package executor

import (
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
)

type ExecutionContext struct {
	QueryPlan     *planner.QueryPlan
	Request       *requests.Request
	Queryers      map[string]queryer.Queryer
	InitilaResult map[string]interface{}
}

type Executor interface {
	Execute(*ExecutionContext) (map[string]interface{}, error)
}
