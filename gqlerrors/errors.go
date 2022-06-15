package gqlerrors

import (
	"strings"

	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	ValidationFailedError = "GRAPHQL_VALIDATION_FAILED"
	UndefinedError        = "UNDEFINED_ERROR"
)

type Location struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

// Error represents a graphql error
type Error struct {
	Extensions map[string]interface{} `json:"extensions"`
	Message    string                 `json:"message"`
	Locations  []Location             `json:"locations,omitempty"`
	Path       []interface{}          `json:"path,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// NewError returns a graphql error with the given code and message
func NewError(code string, err error) *Error {
	return &Error{
		Message: err.Error(),
		Extensions: map[string]interface{}{
			"code": code,
		},
	}
}

// ErrorList represents a list of errors
type ErrorList []*Error

// ExtendErrorList adds provided err as *Error
func ExtendErrorList(errs ErrorList, err error) ErrorList {
	return append(errs, FormatError(err)...)
}

// Error returns a string representation of each error
func (list ErrorList) Error() string {
	acc := make([]string, len(list))

	for i, err := range list {
		acc[i] = err.Error()
	}

	return strings.Join(acc, ". ")
}

func FormatError(err error) ErrorList {
	if err == nil {
		return nil
	}
	switch e := err.(type) {
	case ErrorList:
		var list ErrorList
		for _, innerErr := range e {
			list = append(list, FormatError(innerErr)...)
		}
		return list
	case *Error:
		return ErrorList{e}
	case *gqlerror.Error:
		var locations []Location
		for _, loc := range e.Locations {
			locations = append(locations, Location(loc))
		}
		var path []string
		if e.Path.String() != "" {
			path = strings.Split(e.Path.String(), ".")
		}
		ext := e.Extensions
		if len(ext) == 0 {
			ext = map[string]interface{}{"code": UndefinedError}
		}
		return ErrorList{&Error{
			Extensions: ext,
			Message:    e.Message,
			Locations:  locations,
			Path:       lo.Map(path, func(el string, i int) interface{} { return el }),
		}}
	case gqlerror.List:
		var list ErrorList
		for _, innerErr := range e {
			list = append(list, FormatError(innerErr)...)
		}
		return list
	default:
		return ErrorList{
			NewError(UndefinedError, err),
		}
	}
}
