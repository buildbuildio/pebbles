package planner

import (
	"fmt"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/merger"
	"github.com/buildbuildio/pebbles/requests"

	"github.com/vektah/gqlparser/v2/ast"
)

// PlanningContext contains the necessary information used to plan a query.
type PlanningContext struct {
	Operation  *ast.OperationDefinition
	Request    *requests.Request
	Schema     *ast.Schema
	TypeURLMap merger.TypeURLMap
}

func (pc *PlanningContext) GetURL(typename, fieldname, fburl string) (string, error) {
	if common.IsBuiltinName(fieldname) {
		return fburl, nil
	}
	url, ok := pc.TypeURLMap.Get(typename, fieldname)
	if !ok {
		return "", fmt.Errorf("could not find location for field %s of type %s", fieldname, typename)
	}

	return url, nil
}
