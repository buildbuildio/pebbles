package executor

import (
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
)

type GetParentTypeFromIDFunc func(id interface{}) (string, bool)

type ExecutionContext struct {
	QueryPlan *planner.QueryPlan
	Request   *requests.Request
	Queryers  map[string]queryer.Queryer
	// InitialResult used when working with result from subscription
	InitialResult map[string]interface{}
	// GetParentTypeFromIDFunc is an optional optimization helper.
	// It takes id and return correspondinf parentType, f.e. User
	// If it fails to determine, return false
	// It helps not to query interfaces which are determined to fail
	// f.e. when having id like user_10 and querying node(id: "user_10") (... on Book { id name })
	// it's obvious in advance that result will be null
	GetParentTypeFromIDFunc GetParentTypeFromIDFunc
}

type Executor interface {
	Execute(*ExecutionContext) (map[string]interface{}, error)
}
