package planner

import (
	"encoding/json"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/format"
	"github.com/samber/lo"

	"github.com/vektah/gqlparser/v2/ast"
)

type Planner interface {
	Plan(*PlanningContext) (*QueryPlan, error)
}

// QueryPlan is a query execution plan
type QueryPlan struct {
	RootSteps   []*QueryPlanStep
	ScrubFields ScrubFields
}

// Step is a single execution step
type QueryPlanStep struct {
	URL            string
	OperationName  *string
	ParentType     string
	SelectionSet   ast.SelectionSet
	InsertionPoint []string
	Then           []*QueryPlanStep

	// artifacts
	QueryString   string
	VariablesList []string
}

// MarshalJSON marshals the step the JSON
func (s *QueryPlanStep) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		URL            string
		ParentType     string
		SelectionSet   string
		OperationName  *string
		InsertionPoint []string
		Then           []*QueryPlanStep
	}{
		URL:            s.URL,
		ParentType:     s.ParentType,
		SelectionSet:   format.DebugFormatSelectionSetWithArgs(s.SelectionSet),
		OperationName:  s.OperationName,
		InsertionPoint: s.InsertionPoint,
		Then:           s.Then,
	})
}

func (s *QueryPlanStep) SetVariablesList() *QueryPlanStep {
	args := lo.Uniq(getVariablesList(s.SelectionSet))

	if len(args) == 0 {
		args = nil
	}

	s.VariablesList = args
	return s
}

func (s *QueryPlanStep) SetQuery() *QueryPlanStep {
	s.QueryString = format.FormatSelectionSetWithArgs(s.SelectionSet, s.OperationName)
	return s
}

func getVariablesList(s ast.SelectionSet) []string {
	var args []string
	for _, f := range common.SelectionSetToFields(s, nil) {
		for _, a := range f.Arguments {
			if a.Value != nil {
				args = append(args, a.Value.Raw)
			}
		}

		if f.SelectionSet != nil {
			args = append(args, getVariablesList(f.SelectionSet)...)
		}
	}
	return args
}
