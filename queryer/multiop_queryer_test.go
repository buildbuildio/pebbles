package queryer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/buildbuildio/pebbles/requests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ClosingBuffer struct {
	*bytes.Buffer
}

func (cb *ClosingBuffer) Close() error {
	//we don't actually have to do anything here, since the buffer is
	//and the error is initialized to no-error
	return nil
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestNewMultiOpQueryer(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 100)

	// make sure the queryer config is all correct
	assert.Equal(t, "foo", queryer.URL())
	assert.Equal(t, 100, queryer.maxBatchSize)
}

func TestMultiOpQueryeErrors(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 1)

	queryer.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			defer req.Body.Close()

			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body:   ioutil.NopCloser(bytes.NewBufferString(`[{"errors": [{"message": "myError"}], "data": null}]`)),
				Header: make(http.Header),
			}
		}),
	})

	query := "{ called }"

	// query
	_, err := queryer.Query(
		[]*requests.Request{{Query: query}},
	)
	assert.EqualError(t, err, "myError")
}

func TestMultiOpQueryerBadResponseStatus(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 1)

	queryer.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			defer req.Body.Close()

			return &http.Response{
				StatusCode: 400,
				// Send response to be tested
				Body:   ioutil.NopCloser(bytes.NewBufferString(`{"error": true}`)),
				Header: make(http.Header),
			}
		}),
	})

	query := "{ called }"

	// query
	_, err := queryer.Query(
		[]*requests.Request{{Query: query}},
	)
	assert.Error(t, err)
}

func TestMultiOpQueryerBadResponseBody(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 1)

	queryer.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			defer req.Body.Close()

			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body:   ioutil.NopCloser(bytes.NewBufferString(`{"error": true`)),
				Header: make(http.Header),
			}
		}),
	})

	query := "{ called }"

	// query
	_, err := queryer.Query(
		[]*requests.Request{{Query: query}},
	)
	assert.Error(t, err)
}

func TestMultiOpQueryerQuery(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 3)

	ctx := context.WithValue(context.Background(), "key", "value")

	queryer.WithContext(ctx)

	queryer.WithMiddlewares([]RequestMiddleware{func(r *http.Request) error {
		// check context
		c := r.Context()
		require.Equal(t, "value", c.Value("key"))
		// set header to test later
		r.Header.Set("test", "test")
		return nil
	}})

	queryer.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			defer req.Body.Close()
			buf := &bytes.Buffer{}

			buf.ReadFrom(req.Body)
			body := ""

			var reqBody []interface{}
			json.Unmarshal(buf.Bytes(), &reqBody)

			assert.Equal(t, "test", req.Header.Get("test"))

			for i := 0; i < len(reqBody); i++ {
				body += fmt.Sprintf(`{ "data": { "called": %d } },`, i)
			}

			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body: ioutil.NopCloser(bytes.NewBufferString(fmt.Sprintf(`[
					%s
				]`, body[:len(body)-1]))),
				Header: make(http.Header),
			}
		}),
	})

	// the query we will be batching
	query := "{ called }"

	var severalInputs []*requests.Request
	var expectedResult []map[string]interface{}

	for i := 0; i < 10; i++ {
		severalInputs = append(severalInputs, &requests.Request{Query: query})
		expectedResult = append(expectedResult, map[string]interface{}{"called": float64(i % 3)})
	}

	// query
	results, err := queryer.Query(
		severalInputs,
	)
	assert.NoError(t, err)
	assert.Len(t, results, 10)
	assert.EqualValues(t, expectedResult, results)
}

func TestMultiOpQueryerQueryFile(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 3)

	queryer.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			defer req.Body.Close()
			body, _ := ioutil.ReadAll(req.Body)
			sbody := string(body)
			ok := strings.Contains(sbody, `{"query":"{ called }","variables":{"someFile":null,"stringParam":"hello world"},"operationName":null}`) &&
				strings.Contains(sbody, `Content-Disposition: form-data; name="operations"`) &&
				strings.Contains(sbody, `{"0":["variables.someFile"]}`) &&
				strings.Contains(sbody, `my file content`)
			if !ok {
				return &http.Response{
					StatusCode: 500,
					Header:     make(http.Header),
				}
			}
			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body: ioutil.NopCloser(bytes.NewBufferString(fmt.Sprintf(`
					{"data": {"called": "file"}}
				`))),
				Header: make(http.Header),
			}
		}),
	})

	// the query we will be batching
	query := "{ called }"

	buf := &ClosingBuffer{bytes.NewBuffer([]byte("my file content"))}

	upload := &requests.Upload{File: buf, FileName: "file"}

	input := &requests.Request{
		Query: query,
		Variables: map[string]interface{}{
			"stringParam": "hello world",
			"someFile":    upload,
		},
	}

	// query
	results, err := queryer.Query(
		[]*requests.Request{input},
	)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMultiOpQueryerQueryFileError(t *testing.T) {
	queryer := NewMultiOpQueryer("foo", 3)

	queryer.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			defer req.Body.Close()
			return &http.Response{
				StatusCode: 200,
				// Send response to be tested
				Body:   ioutil.NopCloser(bytes.NewBufferString(`{"errors": [{"message": "myError"}], "data": null}`)),
				Header: make(http.Header),
			}
		}),
	})

	// the query we will be batching
	query := "{ called }"

	buf := &ClosingBuffer{bytes.NewBuffer([]byte("my file content"))}

	upload := &requests.Upload{File: buf, FileName: "file"}

	input := &requests.Request{
		Query: query,
		Variables: map[string]interface{}{
			"someFile": upload,
		},
	}

	// query
	_, err := queryer.Query(
		[]*requests.Request{input},
	)
	assert.EqualError(t, err, "myError")
}
