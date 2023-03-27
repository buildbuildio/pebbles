package requests

import "github.com/buildbuildio/pebbles/gqlerrors"

type Responses []Response

type Response struct {
	Errors gqlerrors.ErrorList    `json:"errors"`
	Data   map[string]interface{} `json:"data"`
}
