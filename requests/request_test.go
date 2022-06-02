package requests

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSingleRequest(t *testing.T) {
	buf := &bytes.Buffer{}

	buf.WriteString(`{"operationName": "test", "query": "query test { test }", "variables": null}`)

	r := httptest.NewRequest("POST", "/", buf)

	actual, err := Parse(r)
	assert.NoError(t, err)

	assert.NotNil(t, actual.Requests[0].Original)

	mustCheckEqual(t, actual, `{
		"IsBatchMode": false, 
		"Requests": [{"query": "query test { test }", "operationName": "test", "variables": null}]
	}`)
}

func TestParseMultipleRequests(t *testing.T) {
	buf := &bytes.Buffer{}

	buf.WriteString(`[{"operationName": null, "query": "{ test }", "variables": null}, {"operationName": null, "query": "{ other }", "variables": null}]`)

	r := httptest.NewRequest("POST", "/", buf)

	actual, err := Parse(r)
	assert.NoError(t, err)

	for _, v := range actual.Requests {
		assert.NotNil(t, v.Original)
	}

	mustCheckEqual(t, actual, `{
		"IsBatchMode": true, 
		"Requests": [
			{"query": "{ test }", "operationName": null, "variables": null},
			{"query": "{ other }", "operationName": null, "variables": null}
		]
	}`)
}

func TestParseInvalidRequest(t *testing.T) {
	buf := &bytes.Buffer{}

	for _, b := range []string{`{"operationName": "test", "query": "", "variables": null}`, `[{"operationName": "test", "query": "", "variables": null}]`} {
		buf.WriteString(b)

		r := httptest.NewRequest("POST", "/", buf)

		_, err := Parse(r)
		assert.Error(t, err)
		buf.Reset()
	}
}

func TestParseFileSingleRequest(t *testing.T) {
	buf := &bytes.Buffer{}

	writer := multipart.NewWriter(buf)

	part, err := writer.CreateFormFile("0", "test.txt")
	require.NoError(t, err)

	part.Write([]byte("hello world!"))

	wOps, err := writer.CreateFormField("operations")
	require.NoError(t, err)

	wOps.Write([]byte(`{"query": "mutation($file: Upload!) {singleUpload(file: $file) {id}}", "variables": { "file": null }}`))

	wMap, err := writer.CreateFormField("map")
	require.NoError(t, err)

	wMap.Write([]byte(`{ "0": ["variables.file"] }`))

	writer.Close()

	r := httptest.NewRequest("POST", "/", buf)

	r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

	actual, err := Parse(r)
	require.NoError(t, err)

	upload := actual.Requests[0].Variables["file"].(*Upload)

	delete(actual.Requests[0].Variables, "file")

	assert.Equal(t, "test.txt", upload.FileName)

	buf.Reset()

	buf.ReadFrom(upload.File)

	assert.Equal(t, "hello world!", buf.String())

	mustCheckEqual(t, actual, `{
		"IsBatchMode": false, 
		"Requests": [
			{"query": "mutation($file: Upload!) {singleUpload(file: $file) {id}}", "operationName": null, "variables": {}}
		]
	}`)
}

func TestParseFileMultipleSingleRequest(t *testing.T) {
	buf := &bytes.Buffer{}

	writer := multipart.NewWriter(buf)

	fpart, err := writer.CreateFormFile("0", "test.txt")
	require.NoError(t, err)

	fpart.Write([]byte("hello world!"))

	spart, err := writer.CreateFormFile("1", "other.txt")
	require.NoError(t, err)

	spart.Write([]byte("sweet dreams!"))

	wOps, err := writer.CreateFormField("operations")
	require.NoError(t, err)

	wOps.Write([]byte(`{ "query": "mutation($files: [Upload!]!) { multipleUpload(files: $files) { id } }", "variables": { "files": [null, null] } }`))

	wMap, err := writer.CreateFormField("map")
	require.NoError(t, err)

	wMap.Write([]byte(`{ "0": ["variables.files.0"], "1": ["variables.files.1"] }`))

	writer.Close()

	r := httptest.NewRequest("POST", "/", buf)

	r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

	actual, err := Parse(r)
	require.NoError(t, err)

	files := actual.Requests[0].Variables["files"].([]interface{})

	upload := make([]*Upload, 0)

	for _, f := range files {
		upload = append(upload, f.(*Upload))
	}

	delete(actual.Requests[0].Variables, "files")

	assert.Equal(t, "test.txt", upload[0].FileName)
	assert.Equal(t, "other.txt", upload[1].FileName)

	buf.Reset()

	buf.ReadFrom(upload[0].File)

	assert.Equal(t, "hello world!", buf.String())

	buf.Reset()

	buf.ReadFrom(upload[1].File)

	assert.Equal(t, "sweet dreams!", buf.String())

	mustCheckEqual(t, actual, `{
		"IsBatchMode": false, 
		"Requests": [
			{"query": "mutation($files: [Upload!]!) { multipleUpload(files: $files) { id } }", "operationName": null, "variables": {}}
		]
	}`)
}

func TestParseFileMultipleRequests(t *testing.T) {
	buf := &bytes.Buffer{}

	writer := multipart.NewWriter(buf)

	fpart, err := writer.CreateFormFile("0", "test.txt")
	require.NoError(t, err)

	fpart.Write([]byte("hello world!"))

	spart, err := writer.CreateFormFile("1", "other.txt")
	require.NoError(t, err)

	spart.Write([]byte("sweet dreams!"))

	tpart, err := writer.CreateFormFile("2", "another.txt")
	require.NoError(t, err)

	tpart.Write([]byte("another!"))

	wOps, err := writer.CreateFormField("operations")
	require.NoError(t, err)

	wOps.Write([]byte(`[{ "query": "mutation ($file: Upload!) { singleUpload(file: $file) { id } }", "variables": { "file": null } }, { "query": "mutation($files: [Upload!]!) { multipleUpload(files: $files) { id } }", "variables": { "files": [null, null] } }]`))

	wMap, err := writer.CreateFormField("map")
	require.NoError(t, err)

	wMap.Write([]byte(`{ "0": ["0.variables.file"], "1": ["1.variables.files.0"], "2": ["1.variables.files.1"] }`))

	writer.Close()

	r := httptest.NewRequest("POST", "/", buf)

	r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

	actual, err := Parse(r)
	require.NoError(t, err)

	// first request
	upload := actual.Requests[0].Variables["file"].(*Upload)

	delete(actual.Requests[0].Variables, "file")

	assert.Equal(t, "test.txt", upload.FileName)

	buf.Reset()

	buf.ReadFrom(upload.File)

	assert.Equal(t, "hello world!", buf.String())

	files := actual.Requests[1].Variables["files"].([]interface{})

	// second request

	uploads := make([]*Upload, 0)

	for _, f := range files {
		uploads = append(uploads, f.(*Upload))
	}

	delete(actual.Requests[1].Variables, "files")

	assert.Equal(t, "other.txt", uploads[0].FileName)
	assert.Equal(t, "another.txt", uploads[1].FileName)

	buf.Reset()

	buf.ReadFrom(uploads[0].File)

	assert.Equal(t, "sweet dreams!", buf.String())

	buf.Reset()

	buf.ReadFrom(uploads[1].File)

	assert.Equal(t, "another!", buf.String())

	mustCheckEqual(t, actual, `{
		"IsBatchMode": true, 
		"Requests": [
			{"query": "mutation ($file: Upload!) { singleUpload(file: $file) { id } }", "operationName": null, "variables": {}},
			{"query": "mutation($files: [Upload!]!) { multipleUpload(files: $files) { id } }", "operationName": null, "variables": {}}
		]
	}`)
}

func TestParseFileSingleRequestWrongMap(t *testing.T) {
	for _, varMap := range []string{
		"",
		`{}`,
		`{ "1": ["variables.file"] }`,
		`{ "0": ["wrong.file"] }`,
		`{ "0": ["variables.file1"] }`,
		`{ "0": ["0.variables.file"] }`,
	} {
		buf := &bytes.Buffer{}

		writer := multipart.NewWriter(buf)

		part, err := writer.CreateFormFile("0", "test.txt")
		require.NoError(t, err)

		part.Write([]byte("hello world!"))

		wOps, err := writer.CreateFormField("operations")
		require.NoError(t, err)

		wOps.Write([]byte(`{"query": "mutation($file: Upload!) {singleUpload(file: $file) {id}}", "variables": { "file": null }}`))

		wMap, err := writer.CreateFormField("map")
		require.NoError(t, err)

		wMap.Write([]byte(varMap))

		writer.Close()

		r := httptest.NewRequest("POST", "/", buf)

		r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

		_, err = Parse(r)
		assert.Error(t, err, varMap)
	}
}

func TestParseFileMultipleRequestsWrongMap(t *testing.T) {
	for _, varMap := range []string{
		"",
		`{}`,
		`{ "1": ["variables.file"] }`,
		`{ "0": ["variables.file.1"], "1": ["variables.files.0.1"], "2": ["variables.files.1.1"] }`,
		`{ "0": ["variables.file"], "1": ["variables.files.0"], "2": ["variables.files.1"] }`,
	} {
		buf := &bytes.Buffer{}

		writer := multipart.NewWriter(buf)

		fpart, err := writer.CreateFormFile("0", "test.txt")
		require.NoError(t, err)

		fpart.Write([]byte("hello world!"))

		spart, err := writer.CreateFormFile("1", "other.txt")
		require.NoError(t, err)

		spart.Write([]byte("sweet dreams!"))

		tpart, err := writer.CreateFormFile("2", "another.txt")
		require.NoError(t, err)

		tpart.Write([]byte("another!"))

		wOps, err := writer.CreateFormField("operations")
		require.NoError(t, err)

		wOps.Write([]byte(`[{ "query": "mutation ($file: Upload!) { singleUpload(file: $file) { id } }", "variables": { "file": null } }, { "query": "mutation($files: [Upload!]!) { multipleUpload(files: $files) { id } }", "variables": { "files": [null, null] } }]`))

		wMap, err := writer.CreateFormField("map")
		require.NoError(t, err)

		wMap.Write([]byte(varMap))

		writer.Close()

		r := httptest.NewRequest("POST", "/", buf)

		r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

		_, err = Parse(r)
		assert.Error(t, err, varMap)
	}
}

func TestParseFileSingleRequestWrongOperations(t *testing.T) {
	for _, ops := range []string{
		"",
		`{}`,
		`{"query": "mutation($file: Upload!) {singleUpload(file: $file) {id}}", "variables": { "file": "file" }}`,
	} {
		buf := &bytes.Buffer{}

		writer := multipart.NewWriter(buf)

		part, err := writer.CreateFormFile("0", "test.txt")
		require.NoError(t, err)

		part.Write([]byte("hello world!"))

		wOps, err := writer.CreateFormField("operations")
		require.NoError(t, err)

		wOps.Write([]byte(ops))

		wMap, err := writer.CreateFormField("map")
		require.NoError(t, err)

		wMap.Write([]byte(`{ "0": ["variables.file"] }`))

		writer.Close()

		r := httptest.NewRequest("POST", "/", buf)

		r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

		_, err = Parse(r)
		assert.Error(t, err, ops)
	}
}

func TestParseFileSingleRequestMissingFile(t *testing.T) {

	buf := &bytes.Buffer{}

	writer := multipart.NewWriter(buf)

	wOps, err := writer.CreateFormField("operations")
	require.NoError(t, err)

	wOps.Write([]byte(`{"query": "mutation($file: Upload!) {singleUpload(file: $file) {id}}", "variables": { "file": null }}`))

	wMap, err := writer.CreateFormField("map")
	require.NoError(t, err)

	wMap.Write([]byte(`{ "0": ["variables.file"] }`))

	writer.Close()

	r := httptest.NewRequest("POST", "/", buf)

	r.Header.Add("Content-Type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))

	_, err = Parse(r)
	assert.Error(t, err)

}
