package merger

import (
	"github.com/buildbuildio/pebbles/common"

	"github.com/samber/lo"
	"github.com/vektah/gqlparser/v2/ast"
)

type TypeProps struct {
	// Fields is map[filedname]url
	Fields map[string]string
	// True if type implements node
	IsImplementsNode bool
}

// TypeURLMap represents typename:fieldname:url mapping
type TypeURLMap map[string]*TypeProps

func (t TypeURLMap) GetURLs() []string {
	u := make(map[string]struct{})
	for _, v := range t {
		for _, vv := range v.Fields {
			if _, ok := u[vv]; ok {
				continue
			}
			u[vv] = struct{}{}
		}
	}

	var urls []string

	for url := range u {
		urls = append(urls, url)
	}

	return urls
}

func (t TypeURLMap) Set(typename, fieldname, url string) {
	// not setting node interface fields
	if fieldname == common.IDFieldName {
		return
	}

	if t[typename] == nil {
		t[typename] = &TypeProps{Fields: make(map[string]string)}
	}

	t[typename].Fields[fieldname] = url
}

func (t TypeURLMap) SetTypeIsImplementsNode(typename string) {
	if t[typename] == nil {
		t[typename] = &TypeProps{Fields: make(map[string]string)}
	}

	t[typename].IsImplementsNode = true
}

func (t TypeURLMap) Get(typename, fieldname string) (res string, ok bool) {
	if t[typename] == nil {
		return "", false
	}

	res, ok = t[typename].Fields[fieldname]
	return
}

func (t TypeURLMap) GetTypeIsImplementsNode(typename string) (res bool, ok bool) {
	if t[typename] == nil {
		return false, false
	}

	return t[typename].IsImplementsNode, true
}

func (t TypeURLMap) SetFromSchema(schema map[string]*ast.Definition, url string) {
	for k, v := range schema {
		// no use for such data
		if v.Kind != ast.Object || common.IsBuiltinName(k) {
			continue
		}

		iin := lo.Contains(v.Interfaces, common.NodeInterfaceName)
		if iin {
			t.SetTypeIsImplementsNode(k)
		}

		for _, f := range v.Fields {
			if common.IsBuiltinName(f.Name) || isNodeField(f) {
				continue
			}

			t.Set(k, f.Name, url)
		}
	}
}
