package queryer

import (
	"context"
	"net/http"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/samber/lo"
)

type chunkResponse struct {
	Index    int
	Response []map[string]interface{}
}

// RequestMiddleware are functions can be passed to Queryer to affect its internal behavior
type RequestMiddleware func(*http.Request) error

// MultiOpQueryer is a queryer that will batch subsequent query on some interval into a single network request
// to a single target
type MultiOpQueryer struct {
	ctx     context.Context
	url     string
	client  *http.Client
	mdwares []RequestMiddleware

	maxBatchSize int
}

var _ Queryer = &MultiOpQueryer{}

// NewMultiOpQueryer returns a MultiOpQueryer with the provided parameters
func NewMultiOpQueryer(url string, maxBatchSize int) *MultiOpQueryer {
	queryer := &MultiOpQueryer{
		url:          url,
		client:       &http.Client{},
		maxBatchSize: maxBatchSize,
		ctx:          context.Background(),
	}

	// we're done creating the queryer
	return queryer
}

// WithContext sets ctx which will be passed to following http.Request
func (q *MultiOpQueryer) WithContext(ctx context.Context) *MultiOpQueryer {
	q.ctx = ctx
	return q
}

// WithMiddlewares lets the user assign middlewares to the queryer
func (q *MultiOpQueryer) WithMiddlewares(mwares []RequestMiddleware) *MultiOpQueryer {
	q.mdwares = mwares
	return q
}

// WithHTTPClient lets the user configure the client to use when making network requests
func (q *MultiOpQueryer) WithHTTPClient(client *http.Client) *MultiOpQueryer {
	q.client = client
	return q
}

func (q *MultiOpQueryer) URL() string {
	return q.url
}

func (q *MultiOpQueryer) Query(inputs []*requests.Request) ([]map[string]interface{}, error) {
	// fit in max batch size
	lInputs := len(inputs)
	if lInputs <= q.maxBatchSize {
		return q.queryBatch(inputs)
	}

	// divide into smaller batches
	chunks := lInputs/q.maxBatchSize + 1

	res, err := common.AsyncMapReduce(
		lo.Range(chunks),
		make([]map[string]interface{}, len(inputs)),
		func(i int) (*chunkResponse, error) {
			var inputsSlice []*requests.Request
			if (i+1)*q.maxBatchSize > lInputs {
				inputsSlice = inputs[i*q.maxBatchSize:]
			} else {
				inputsSlice = inputs[i*q.maxBatchSize : (i+1)*q.maxBatchSize]
			}

			res, err := q.queryBatch(inputsSlice)
			if err != nil {
				return nil, err
			}

			return &chunkResponse{
				Index:    i,
				Response: res,
			}, nil
		},
		func(acc []map[string]interface{}, value *chunkResponse) []map[string]interface{} {
			i := value.Index
			tail := value.Response
			if (i+1)*q.maxBatchSize < lInputs {
				tail = append(tail, acc[(i+1)*q.maxBatchSize:]...)
			}

			acc = append(acc[0:i*q.maxBatchSize], tail...)
			return acc
		},
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}

// queryBatch executes provided inputs in single response
func (q *MultiOpQueryer) queryBatch(inputs []*requests.Request) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, len(inputs))
	var toFetchIndexes []int
	var inputsToFetch []*requests.Request

	for i, input := range inputs {
		// check if query contains attached files
		resp, err := q.fetchFile(input)
		if err != nil {
			return nil, err
		}
		// not a file
		if resp == nil {
			inputsToFetch = append(inputsToFetch, input)
			toFetchIndexes = append(toFetchIndexes, i)
			continue
		}

		if len(resp.Errors) != 0 {
			return nil, resp.Errors
		}

		results[i] = resp.Data
	}

	// all inputs were files
	if len(inputsToFetch) == 0 {
		return results, nil
	}

	resps, err := q.fetch(inputsToFetch)
	if err != nil {
		return nil, err
	}

	// format the result as needed
	for i, resp := range resps {
		if len(resp.Errors) != 0 {
			return nil, resp.Errors
		}
		results[toFetchIndexes[i]] = resp.Data
	}

	return results, nil
}
