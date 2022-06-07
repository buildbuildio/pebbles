package pebbles

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/executor"
	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/introspection"
	"github.com/buildbuildio/pebbles/merger"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/playground"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/samber/lo"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type QueryerFactory func(*planner.PlanningContext, string) queryer.Queryer

type Gateway struct {
	schema                   *ast.Schema
	typeURLMap               merger.TypeURLMap
	executor                 executor.Executor
	planner                  planner.Planner
	merger                   merger.Merger
	remoteSchemaIntrospector introspection.RemoteSchemaIntrospector
	queryerFactory           QueryerFactory
	playgroundProvider       playground.PlaygroundProvider
}

type GatewayOption func(*Gateway)

func WithExecutor(e executor.Executor) GatewayOption {
	return func(g *Gateway) {
		g.executor = e
	}
}

func WithRemoteSchemaIntrospector(i introspection.RemoteSchemaIntrospector) GatewayOption {
	return func(g *Gateway) {
		g.remoteSchemaIntrospector = i
	}
}

func WithMerger(m merger.Merger) GatewayOption {
	return func(g *Gateway) {
		g.merger = m
	}
}

func WithPlanner(p planner.Planner) GatewayOption {
	return func(g *Gateway) {
		g.planner = p
	}
}

func WithDefaultPlayground() GatewayOption {
	return func(g *Gateway) {
		var pp playground.DefaultPlayground
		g.playgroundProvider = pp
	}
}

func WithPlaygroundProvider(pp playground.PlaygroundProvider) GatewayOption {
	return func(g *Gateway) {
		g.playgroundProvider = pp
	}
}

func WithQueryerFactory(f QueryerFactory) GatewayOption {
	return func(g *Gateway) {
		g.queryerFactory = f
	}
}

func NewGateway(urls []string, options ...GatewayOption) (*Gateway, error) {
	g := new(Gateway)

	for _, optionFunc := range options {
		optionFunc(g)
	}

	if g.planner == nil {
		var p planner.SequentialPlanner
		g.planner = p
	}

	if g.executor == nil {
		var e executor.ParallelExecutor
		g.executor = e
	}

	if g.remoteSchemaIntrospector == nil {
		g.remoteSchemaIntrospector = &introspection.ParallelRemoteSchemaIntrospector{
			Factory: func(url string) queryer.Queryer {
				return queryer.NewMultiOpQueryer(url, 1)
			},
		}
	}

	if g.merger == nil {
		var m merger.ExtendMergerFunc
		g.merger = m
	}

	if g.queryerFactory == nil {
		g.queryerFactory = func(
			ctx *planner.PlanningContext,
			url string,
		) queryer.Queryer {
			return queryer.NewMultiOpQueryer(
				url, 3000,
			).WithHTTPClient(
				http.DefaultClient,
			).WithContext(
				ctx.Request.Original.Context(),
			)
		}
	}

	// run introspection query against passed urls
	schemas, err := g.remoteSchemaIntrospector.IntrospectRemoteSchemas(urls...)
	if err != nil {
		return nil, fmt.Errorf("unable to introspect remote schemas: %w", err)
	}

	mergeInputs := make([]*merger.MergeInput, len(schemas))
	for i := 0; i < len(schemas); i++ {
		mergeInputs[i] = &merger.MergeInput{
			Schema: schemas[i],
			URL:    urls[i],
		}
	}

	// merge schemas into one
	mr, err := g.merger.Merge(mergeInputs)
	if err != nil {
		return nil, fmt.Errorf("unable to merge schemas: %w", err)
	}

	g.schema = mr.Schema
	g.typeURLMap = mr.TypeURLMap

	return g, nil
}

type Result struct {
	Errors gqlerrors.ErrorList    `json:"errors,omitempty"`
	Data   map[string]interface{} `json:"data"`

	index int `json:"-"`
}

type Results []*Result

func (rs Results) Emit(w http.ResponseWriter, isBatch bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	e := json.NewEncoder(w)
	if isBatch {
		e.Encode(rs)
	} else {
		e.Encode(rs[0])
	}

}

// QueryHandler returns a http.HandlerFunc that should be used as the
// primary endpoint for the gateway API. The endpoint will respond
// to queries on POST requests. POST requests can either be
// a single object with { query, variables, operationName } or a list
// of that object.
func (g *Gateway) queryHandler(w http.ResponseWriter, r *http.Request) {
	rs, err := requests.Parse(r)
	if err != nil {
		emitError(
			w,
			http.StatusUnprocessableEntity,
			err,
		)
		return
	}

	results, _ := common.AsyncMapReduce(
		lo.Range(len(rs.Requests)),
		make(Results, len(rs.Requests)),
		func(index int) (*Result, error) {
			request := rs.Requests[index]
			// the result of the operation
			result := make(map[string]interface{})

			query, qerr := gqlparser.LoadQuery(g.schema, request.Query)
			if qerr != nil {
				return &Result{
					Errors: gqlerrors.FormatError(qerr),
					Data:   nil,

					index: index,
				}, nil
			}

			var operation *ast.OperationDefinition
			if request.OperationName != nil {
				operation = query.Operations.ForName(*request.OperationName)
			} else if len(query.Operations) == 1 && query.Operations[0].Name == "" {
				operation = query.Operations[0]
			}

			if operation == nil {
				var err error
				if request.OperationName != nil {
					err = fmt.Errorf(
						"unable to extract query for operation %s",
						*request.OperationName,
					)
				} else {
					err = errors.New("many queries provided, but no operationName")
				}
				return &Result{
					Errors: gqlerrors.ErrorList{
						gqlerrors.NewError(
							gqlerrors.ValidationFailedError,
							err,
						),
					},
					Data: nil,

					index: index,
				}, nil
			}

			planningContext := &planner.PlanningContext{
				Request:    request,
				Operation:  operation,
				Schema:     g.schema,
				TypeURLMap: g.typeURLMap,
			}

			// get the plan for specific query
			plan, err := g.planner.Plan(planningContext)
			if err != nil {
				return &Result{
					Errors: gqlerrors.ErrorList{
						gqlerrors.NewError(gqlerrors.ValidationFailedError, err),
					},
					Data: nil,

					index: index,
				}, nil
			}

			introspectionRes := g.parseIntrospectionQuery(plan, request)
			if introspectionRes != nil {
				introspectionRes.index = index
				return introspectionRes, nil
			}

			queryers := g.getQueryers(planningContext, plan.RootSteps)

			// fire the query
			result, err = g.executor.Execute(&executor.ExecutionContext{
				QueryPlan: plan,
				Request:   request,
				Queryers:  queryers,
			})

			plan.ScrubFields.Clean(result)

			return &Result{
				Errors: gqlerrors.FormatError(err),
				Data:   result,

				index: index,
			}, nil
		},
		func(acc Results, value *Result) Results {
			acc[value.index] = value
			return acc
		},
	)

	// emit the response
	results.Emit(w, rs.IsBatchMode)

}

func (g *Gateway) parseIntrospectionQuery(plan *planner.QueryPlan, request *requests.Request) *Result {
	for _, rs := range plan.RootSteps {
		if rs.URL == common.InternalServiceName {
			ir := &introspection.IntrospectionResolver{
				Variables: request.Variables,
			}

			introspectionFields := ir.ResolveIntrospectionFields(rs.SelectionSet, g.schema)
			if introspectionFields != nil {
				return &Result{
					Data:   introspectionFields,
					Errors: nil,
				}
			}
		}
	}

	return nil
}

func (g *Gateway) getQueryers(planningCtx *planner.PlanningContext, planSteps []*planner.QueryPlanStep) map[string]queryer.Queryer {
	queryers := make(map[string]queryer.Queryer)
	for _, ps := range planSteps {
		if _, ok := queryers[ps.URL]; !ok {
			queryers[ps.URL] = g.queryerFactory(planningCtx, ps.URL)
		}

		if ps.Then != nil {
			childQueryers := g.getQueryers(planningCtx, ps.Then)
			for url, queryer := range childQueryers {
				queryers[url] = queryer
			}
		}
	}

	return queryers
}

func emitError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := map[string]interface{}{
		"data":   nil,
		"errors": gqlerrors.FormatError(err),
	}

	e := json.NewEncoder(w)
	e.Encode(resp)
}

func (g *Gateway) Handler(w http.ResponseWriter, r *http.Request) {
	if g.playgroundProvider != nil {
		// on POSTs, we have to send the request to the graphqlHandler
		if r.Method == http.MethodPost {
			g.queryHandler(w, r)
			return
		}

		// we are not handling a POST request so we have to show the user the playground
		g.playgroundProvider.ServePlayground(w, r)
		return
	}

	g.queryHandler(w, r)
}
