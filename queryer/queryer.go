package queryer

import "github.com/buildbuildio/pebbles/requests"

type Queryer interface {
	Query([]*requests.Request) ([]map[string]interface{}, error)
	Subscribe(*requests.Request, <-chan struct{}, chan *requests.Response) error
	URL() string
}
