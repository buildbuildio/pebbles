package planner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/buildbuildio/pebbles/common"

	"github.com/samber/lo"
)

type ScrubFields map[string]map[string][]string

func (sf ScrubFields) MarshalJSON() ([]byte, error) {
	if sf == nil {
		return json.Marshal(nil)
	}
	res := make(map[string][]string)
	for i, v := range sf {
		for j, vv := range v {
			res[fmt.Sprintf("%s#%s", i, j)] = vv
		}
	}

	return json.Marshal(res)
}

func (sf ScrubFields) hash(path []string) string {
	return strings.Join(path, ".")
}

func (sf ScrubFields) unhash(key string) []string {
	return strings.Split(key, ".")
}

func (sf ScrubFields) Set(path []string, typename, fieldname string) {
	key := sf.hash(path)
	if sf[key] == nil {
		sf[key] = make(map[string][]string)
	}
	sf[key][typename] = lo.Uniq(append(sf[key][typename], fieldname))
}

func (sf ScrubFields) Get(path []string, typename string) []string {
	key := sf.hash(path)
	if sf[key] == nil {
		return nil
	}
	return sf[key][typename]
}

func (sf ScrubFields) Merge(sfs ScrubFields) {
	for i, v := range sfs {
		if sf[i] == nil {
			sf[i] = make(map[string][]string)
		}
		for j, vv := range v {
			sf[i][j] = lo.Uniq(append(sf[i][j], vv...))
		}

	}
}

func (sf ScrubFields) Clean(payload map[string]interface{}) {
	if sf == nil {
		return
	}

	for key, fields := range sf {
		path := sf.unhash(key)
		sf.clean(payload, path, fields)
	}

	return
}

func (sf ScrubFields) clean(payload map[string]interface{}, path []string, fields map[string][]string) bool {
	if len(path) == 0 {
		for typename, fields := range fields {
			if tn, ok := payload[common.TypenameFieldName]; ok && typename != tn {
				continue
			}

			for _, f := range fields {
				delete(payload, f)
			}
			break
		}
		return len(payload) == 0
	}
	p := path[0]
	obj, ok := payload[p]
	if !ok {
		return false
	}

	removeParent := true

	switch v := obj.(type) {
	case map[string]interface{}:
		removeParent = sf.clean(v, path[1:], fields)
	case []interface{}:
		for _, x := range v {
			if vv, ok := x.(map[string]interface{}); ok {
				toCleanParent := sf.clean(vv, path[1:], fields)
				removeParent = removeParent && toCleanParent
			}
		}
		if len(v) == 0 {
			removeParent = false
		}
	case []map[string]interface{}:
		for _, vv := range v {
			toCleanParent := sf.clean(vv, path[1:], fields)
			removeParent = removeParent && toCleanParent
		}
		if len(v) == 0 {
			removeParent = false
		}
	default:
		// case of null objects
		removeParent = false
	}

	if removeParent {
		delete(payload, p)
	}

	return len(payload) == 0
}
