package merger

import (
	"github.com/buildbuildio/pebbles/common"
	"github.com/vektah/gqlparser/v2/ast"
)

type SanitizeNodeMergerFunc func(schemas []*MergeInput) (*MergeResult, error)

func (SanitizeNodeMergerFunc) Merge(inputs []*MergeInput) (*MergeResult, error) {
	var mf ExtendMergerFunc
	res, err := mf.Merge(inputs)
	if err != nil {
		return nil, err
	}

	// remove node from query
	sanitizedFieldList := make(ast.FieldList, 0)
	for _, field := range res.Schema.Query.Fields {
		if field.Name == common.NodeFieldName {
			continue
		}
		sanitizedFieldList = append(sanitizedFieldList, field)
	}

	res.Schema.Query.Fields = sanitizedFieldList
	return res, nil
}
