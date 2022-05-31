package executor

import (
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
)

type ExecutionContext struct {
	// RequestMiddlewares []graphql.NetworkMiddleware
	QueryPlan *planner.QueryPlan
	Request   *requests.Request
	Queryers  map[string]queryer.Queryer
}

type Executor interface {
	Execute(*ExecutionContext) (map[string]interface{}, error)
}
