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

func (qp *QueryPlan) SetComputedValues(ctx *PlanningContext) *QueryPlan {
	for i, step := range qp.RootSteps {
		// set OperationName for root steps if provided
		// by realization there're no operations in sub query
		if ctx.Operation.Name != "" && len(step.InsertionPoint) == 0 {
			step.OperationName = &ctx.Operation.Name
		}

		qp.RootSteps[i] = step.SetComputedValues()
	}

	return qp
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

	// tools
	formatter *format.BufferedFormatter
}

// MarshalJSON marshals the step the JSON. Used for test purposes only
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
		SelectionSet:   format.NewDebugBufferedFormatter().FormatSelectionSet(s.SelectionSet),
		OperationName:  s.OperationName,
		InsertionPoint: s.InsertionPoint,
		Then:           s.Then,
	})
}

// SetComputedValues sets VariablesList and Query for QueryPlanStep. It triggers SetComputedValues on child steps
func (s *QueryPlanStep) SetComputedValues() *QueryPlanStep {
	s = s.setVariablesList().setQuery()
	for i, then := range s.Then {
		s.Then[i] = then.SetComputedValues()
	}
	return s
}

func (s *QueryPlanStep) setVariablesList() *QueryPlanStep {
	args := lo.Uniq(getVariablesList(s.SelectionSet))

	if len(args) == 0 {
		args = nil
	}

	s.VariablesList = args
	return s
}

func (s *QueryPlanStep) setQuery() *QueryPlanStep {
	if s.formatter == nil {
		// fallback for test case scenario
		s.formatter = format.NewBufferedFormatter()
	}
	if s.OperationName != nil {
		s.formatter.WithOperationName(*s.OperationName)
	}
	s.QueryString = s.formatter.FormatSelectionSet(s.SelectionSet)
	return s
}

func getVariablesList(s ast.SelectionSet) []string {
	var args []string
	for _, f := range common.SelectionSetToFields(s, nil) {
		for _, a := range f.Arguments {
			if len(a.Value.Children) > 0 {
				args = append(args, getArgumentListChildrenVariablesList(a.Value.Children)...)
				continue
			}

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

func getArgumentListChildrenVariablesList(childs ast.ChildValueList) []string {
	var args []string
	for _, ch := range childs {
		if len(ch.Value.Children) > 0 {
			args = append(args, getArgumentListChildrenVariablesList(ch.Value.Children)...)
			continue
		}

		if ch.Value != nil {
			args = append(args, ch.Value.Raw)
		}
	}
	return args
}
