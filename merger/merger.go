package merger

import (
	"github.com/vektah/gqlparser/v2/ast"
)

type MergeResult struct {
	Schema     *ast.Schema
	TypeURLMap TypeURLMap
}

type MergeInput struct {
	Schema *ast.Schema
	URL    string
}

// Merger is an interface for structs that are capable of taking a list of schemas and returning something that resembles
// a "merge" of those schemas.
type Merger interface {
	Merge([]*MergeInput) (*MergeResult, error)
}
