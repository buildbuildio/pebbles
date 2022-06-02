package requests

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// Request represents single request send via HTTP
type Request struct {
	Original      *http.Request          `json:"-"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName *string                `json:"operationName"`
}

type File interface {
	io.Reader
	io.Closer
}

// Upload represent file and it's name
type Upload struct {
	File     File
	FileName string
}

// ParseRequestResponse is an resulting object of ParseRequestQuery.
// It contains requests array and indicator, if request was running in batch mode.
type ParseRequestResponse struct {
	Requests    []*Request
	IsBatchMode bool
}

func Parse(r *http.Request) (resp *ParseRequestResponse, finalErr error) {
	defer func() {
		if resp == nil {
			return
		}
		for _, req := range resp.Requests {
			req.Original = r
		}
	}()
	if r.Method != http.MethodPost {
		return nil, errors.New("only POST requests are supported")
	}

	contentType := strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0]

	switch contentType {
	case "text/plain", "application/json", "":
		// read the full request body
		requestBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("encountered error reading body: %s", err)
		}
		resp, finalErr = parseRequest(requestBytes)
		return
	case "multipart/form-data":
		err := r.ParseMultipartForm(32 << 20)
		if err != nil {
			return nil, fmt.Errorf("error parse multipart form data: %s", err)
		}

		requestBytes := []byte(r.Form.Get("operations"))
		resp, err = parseRequest(requestBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse request: %s", err)
		}

		var filePosMap map[string][]string
		if err := json.Unmarshal([]byte(r.Form.Get("map")), &filePosMap); err != nil {
			return nil, fmt.Errorf("error parsing file map: %s", err)
		}

		if len(filePosMap) == 0 {
			return nil, errors.New("file map is empty")
		}

		for filePos, paths := range filePosMap {
			file, header, err := r.FormFile(filePos)
			if err != nil {
				return nil, fmt.Errorf("file with index %s not found: %s", filePos, err)
			}

			upload := &Upload{
				File:     file,
				FileName: header.Filename,
			}

			if err := resp.injectFile(upload, paths); err != nil {
				return nil, err
			}
		}
		return resp, nil
	default:
		return nil, fmt.Errorf("unknown content-type: %s", contentType)
	}
}

// parseRequest takes byte body of request and tries to parse it.
func parseRequest(body []byte) (*ParseRequestResponse, error) {
	if IsBatchMode(body) {
		// multiple objects case
		var multipleRequests []*Request

		if err := json.Unmarshal(body, &multipleRequests); err != nil {
			return nil, fmt.Errorf("unable to parse given request in batch mode: %s", body)
		}

		for _, r := range multipleRequests {
			if r.Query == "" {
				return nil, errors.New("missing query from request")
			}
		}

		return &ParseRequestResponse{
			Requests:    multipleRequests,
			IsBatchMode: true,
		}, nil
	}

	// single object case
	var singleRequest Request
	if err := json.Unmarshal(body, &singleRequest); err != nil {
		return nil, fmt.Errorf("unable to parse given request in single mode: %s", body)
	}

	if singleRequest.Query == "" {
		return nil, errors.New("missing query from request")
	}

	return &ParseRequestResponse{
		Requests:    []*Request{&singleRequest},
		IsBatchMode: false,
	}, nil
}

// injectFile adds file object to variables of respective queries
func (r *ParseRequestResponse) injectFile(upload *Upload, paths []string) error {
	for _, path := range paths {
		var idx = 0
		parts := strings.Split(path, ".")
		// in batch mode idx is hidden inside path
		if r.IsBatchMode {
			idxVal, err := strconv.Atoi(parts[0])
			if err != nil {
				return err
			}
			idx = idxVal
			parts = parts[1:]
		}

		if parts[0] != "variables" {
			return fmt.Errorf("missing keyword variables in path: %s", path)
		}

		if len(parts) < 2 {
			return fmt.Errorf("invalid number of parts in path: %s", path)
		}

		variables := r.Requests[idx].Variables

		// step through the path to find the file variable
		for i := 1; i < len(parts); i++ {
			val, ok := variables[parts[i]]
			if !ok {
				return fmt.Errorf("key not found in variables: %s", parts[i])
			}
			switch v := val.(type) {
			// if the path part is a map, then keep stepping through it
			case map[string]interface{}:
				variables = v
			// if we hit nil, then we have found the variable to replace with the file and have hit the end of parts
			case nil:
				variables[parts[i]] = upload
			// if we find a list then find the the variable to replace at the parts index (supports: [Upload!]!)
			case []interface{}:
				// make sure the path contains another part before looking for an index
				if i+1 >= len(parts) {
					return fmt.Errorf("invalid number of parts in path: %s", path)
				}

				// the next part in the path must be an index (ex: the "2" in: variables.input.files.2)
				index, err := strconv.Atoi(parts[i+1])
				if err != nil {
					return fmt.Errorf("expected numeric index: %s", err)
				}

				// index might not be within the bounds
				if index >= len(v) {
					return fmt.Errorf("file index %d out of bound %d", index, len(v))
				}
				fileVal := v[index]
				if fileVal != nil {
					return fmt.Errorf("expected nil value, got %v", fileVal)
				}
				v[index] = upload

				// skip the final iteration through parts (skips the index definition, ex: the "2" in: variables.input.files.2)
				i++
			default:
				return fmt.Errorf("expected nil value, got %v", v) // possibly duplicate path or path to non-null variable
			}
		}
	}

	return nil
}

func IsBatchMode(body []byte) bool {
	for _, c := range body {
		if c == '[' {
			return true
		}
		if c == '{' {
			return false
		}
	}

	return false
}
