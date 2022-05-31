package queryer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/requests"
)

type Responses []Response

type Response struct {
	Errors gqlerrors.ErrorList    `json:"errors"`
	Data   map[string]interface{} `json:"data"`
}

// SendQuery is responsible for sending the provided payload to the desingated URL
func (q *MultiOpQueryer) sendQueryRequest(payload []byte) ([]byte, error) {
	// construct the initial request we will send to the client
	req, err := http.NewRequest("POST", q.url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	// add ctx to request
	if q.ctx != nil {
		req = req.WithContext(q.ctx)
	}
	// add the current context to the request
	req.Header.Set("Content-Type", "application/json")

	return q.sendRequest(req)
}

// SendMultipart is responsible for sending multipart request to the desingated URL
func (q *MultiOpQueryer) sendMultipartRequest(payload []byte, contentType string) ([]byte, error) {
	// construct the initial request we will send to the client
	req, err := http.NewRequest("POST", q.url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	// add the current context to the request
	req.Header.Set("Content-Type", contentType)

	return q.sendRequest(req)
}

func (q *MultiOpQueryer) sendRequest(request *http.Request) ([]byte, error) {
	// we could have any number of middlewares that we have to go through so
	for _, mdware := range q.mdwares {
		err := mdware(request)
		if err != nil {
			return nil, err
		}
	}

	// fire the response to the queryer's url
	if q.client == nil {
		q.client = &http.Client{}
	}

	resp, err := q.client.Do(request)
	if err != nil {
		return nil, err
	}

	// read the full body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return body, errors.New("response was not successful with status code: " + strconv.Itoa(resp.StatusCode))
	}

	// we're done
	return body, nil
}

func (q *MultiOpQueryer) fetch(inputs []*requests.Request) ([]Response, error) {
	var payload []byte

	bRs, err := json.Marshal(inputs)
	if err != nil {
		return nil, err
	}
	payload = bRs

	// a place to store the results
	results := make(Responses, len(inputs))

	// execute http request
	response, err := q.sendQueryRequest(payload)

	if err != nil {
		return nil, err
	}

	// a place to handle each result
	if err := json.Unmarshal(response, &results); err != nil {
		return nil, err
	}

	// return the results
	return results, nil
}

func (q *MultiOpQueryer) fetchFile(input *requests.Request) (*Response, error) {
	uploadMap := extractFiles(input)

	if uploadMap.Empty() {
		return nil, nil
	}

	bInput, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	body, contentType, err := prepareMultipart(bInput, *uploadMap)
	if err != nil {
		return nil, err
	}

	responseBody, err := q.sendMultipartRequest(body, contentType)
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
