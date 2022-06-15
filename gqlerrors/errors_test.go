package gqlerrors

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestError(t *testing.T) {
	err := NewError("code", errors.New("error"))

	assert.Equal(t, "error", err.Error())
	assert.Equal(t, err.Extensions["code"], "code")

	l := ErrorList{err, err}

	assert.Equal(t, "error. error", l.Error())

}

func TestExtendError(t *testing.T) {
	expected := ErrorList{NewError(UndefinedError, errors.New("test")), NewError(UndefinedError, errors.New("error"))}
	for _, e := range []error{
		NewError(UndefinedError, errors.New("error")),
		ErrorList{NewError(UndefinedError, errors.New("error"))},
		&gqlerror.Error{Message: "error"},
		gqlerror.List{&gqlerror.Error{Message: "error"}},
		errors.New("error"),
	} {
		err := ErrorList{NewError(UndefinedError, errors.New("test"))}
		actual := ExtendErrorList(err, e)
		bExpected, _ := json.Marshal(expected)
		bActual, _ := json.Marshal(actual)
		assert.JSONEq(t, string(bExpected), string(bActual))
	}
}

func TestFormatError(t *testing.T) {
	expected := ErrorList{NewError(UndefinedError, errors.New("error"))}
	for _, e := range []error{
		NewError(UndefinedError, errors.New("error")),
		ErrorList{NewError(UndefinedError, errors.New("error"))},
		&gqlerror.Error{Message: "error"},
		gqlerror.List{&gqlerror.Error{Message: "error"}},
		errors.New("error"),
	} {
		actual := FormatError(e)
		bExpected, _ := json.Marshal(expected)
		bActual, _ := json.Marshal(actual)
		assert.JSONEq(t, string(bExpected), string(bActual))
	}
}

func TestFormatErrorNilValue(t *testing.T) {
	actual := FormatError(nil)
	assert.Nil(t, actual)
}

func TestFormatErrorComplicatedGQLError(t *testing.T) {
	err := &gqlerror.Error{
		Message:   "hello",
		Locations: []gqlerror.Location{{Line: 1, Column: 1}},
		Path:      ast.Path{ast.PathName("path")},
		Extensions: map[string]interface{}{
			"smth": 42,
		},
	}
	actual := FormatError(err)

	bActual, _ := json.Marshal(actual)

	assert.JSONEq(t, `[{"message": "hello", "path": ["path"], "extensions": {"smth": 42}, "locations": [{"line": 1, "column": 1}]}]`, string(bActual))
}

func TestErrorUnmarshall(t *testing.T) {
	errMsg := `[{"message":"Error during request","path":["somePath","results",2,"items"]}]`

	var err ErrorList

	errJson := json.Unmarshal([]byte(errMsg), &err)

	assert.NoError(t, errJson)

	assert.Equal(t, "Error during request", err[0].Message)
}
