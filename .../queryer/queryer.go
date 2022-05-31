package queryer

import "github.com/buildbuildio/pebbles/requests"

type Queryer interface {
	Query([]*requests.Request) ([]map[string]interface{}, error)
	URL() string
}
