package executor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestResultFindInsertionPointRootList(t *testing.T) {
	// in this example, the step before would have just resolved (need to be inserted at)
	// ["users", "photoGallery"]. There would be an id field underneath each photo in the list
	// of users.photoGallery

	// we want the list of insertion points that point to
	planInsertionPoint := []string{"users", "photoGallery", "likedBy"}

	// pretend we are in the middle of stitching a larger object
	startingPoint := [][]string{}

	// there are 6 total insertion points in this example
	finalInsertionPoint := [][]string{
		// photo 0 is liked by 2 users
		{"users:0", "photoGallery:0", "likedBy:0#1"},
		{"users:0", "photoGallery:0", "likedBy:1#2"},
		// photo 1 is liked by 3 users
		{"users:0", "photoGallery:1", "likedBy:0#3"},
		{"users:0", "photoGallery:1", "likedBy:1#4"},
		{"users:0", "photoGallery:1", "likedBy:2#5"},
		// photo 2 is liked by 1 user
		{"users:0", "photoGallery:2", "likedBy:0#6"},
	}

	// the selection we're going to make
	stepSelectionSet := ast.SelectionSet{
		&ast.Field{
			Name: "users",
			Definition: &ast.FieldDefinition{
				Type: ast.ListType(ast.NamedType("User", nil), nil),
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: "photoGallery",
					Definition: &ast.FieldDefinition{
						Type: ast.ListType(ast.NamedType("Photo", nil), nil),
					},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "likedBy",
							Definition: &ast.FieldDefinition{
								Type: ast.ListType(ast.NamedType("User", nil), nil),
							},
							SelectionSet: ast.SelectionSet{
								&ast.Field{
									Name: "totalLikes",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("Int", nil),
									},
								},
								&ast.Field{
									Name: "id",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("ID", nil),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// the result of the step
	result := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"photoGallery": []interface{}{
					map[string]interface{}{
						"likedBy": []interface{}{
							map[string]interface{}{
								"totalLikes": 10,
								"id":         "1",
							},
							map[string]interface{}{
								"totalLikes": 10,
								"id":         "2",
							},
						},
					},
					map[string]interface{}{
						"likedBy": []interface{}{
							map[string]interface{}{
								"totalLikes": 10,
								"id":         "3",
							},
							map[string]interface{}{
								"totalLikes": 10,
								"id":         "4",
							},
							map[string]interface{}{
								"totalLikes": 10,
								"id":         "5",
							},
						},
					},
					map[string]interface{}{
						"likedBy": []interface{}{
							map[string]interface{}{
								"totalLikes": 10,
								"id":         "6",
							},
						},
					},
					map[string]interface{}{
						"likedBy": []interface{}{},
					},
				},
			},
		},
	}

	generatedPoint, err := FindInsertionPoints(planInsertionPoint, stepSelectionSet, result, startingPoint)
	assert.NoError(t, err)

	assert.Equal(t, finalInsertionPoint, generatedPoint)
}

func TestResultFindInsertionPointStitchIntoObject(t *testing.T) {
	// we want the list of insertion points that point to
	planInsertionPoint := []string{"users", "photoGallery", "author"}

	// pretend we are in the middle of stitching a larger object
	startingPoint := [][]string{{"users:0"}}

	// there are 3 total insertion points in this example
	finalInsertionPoint := [][]string{
		{"users:0", "photoGallery:0", "author#1"},
		{"users:0", "photoGallery:1", "author#2"},
		{"users:0", "photoGallery:2", "author#3"},
	}

	// the selection we're going to make
	stepSelectionSet := ast.SelectionSet{
		&ast.Field{
			Name: "photoGallery",
			Definition: &ast.FieldDefinition{
				Type: ast.ListType(ast.NamedType("Photo", nil), nil),
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: "author",
					Definition: &ast.FieldDefinition{
						Type: ast.NamedType("User", nil),
					},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "totalLikes",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("Int", nil),
							},
						},
						&ast.Field{
							Name: "id",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("ID", nil),
							},
						},
					},
				},
			},
		},
	}

	// the result of the step
	result := map[string]interface{}{
		"photoGallery": []interface{}{
			map[string]interface{}{
				"author": map[string]interface{}{
					"id": "1",
				},
			},
			map[string]interface{}{
				"author": map[string]interface{}{
					"id": "2",
				},
			},
			map[string]interface{}{
				"author": map[string]interface{}{
					"id": "3",
				},
			},
		},
	}

	generatedPoint, err := FindInsertionPoints(planInsertionPoint, stepSelectionSet, result, startingPoint)
	assert.NoError(t, err)

	assert.Equal(t, finalInsertionPoint, generatedPoint)
}

func TestResultFindInsertionPointWorkOnNil(t *testing.T) {
	// we want the list of insertion points that point to
	planInsertionPoint := []string{"post", "author"}
	expected := [][]string{}

	result := map[string]interface{}{
		"post": map[string]interface{}{
			"author": nil,
		},
	}

	// the selection we're going to make
	stepSelectionSet := ast.SelectionSet{
		&ast.Field{
			Name: "user",
			Definition: &ast.FieldDefinition{
				Type: ast.ListType(ast.NamedType("Photo", nil), nil),
			},
			SelectionSet: ast.SelectionSet{
				&ast.Field{
					Name: "firstName",
					Definition: &ast.FieldDefinition{
						Type: ast.NamedType("String", nil),
					},
				},
			},
		},
	}

	generatedPoint, err := FindInsertionPoints(planInsertionPoint, stepSelectionSet, result, [][]string{})
	assert.NoError(t, err)

	assert.Equal(t, expected, generatedPoint)
}

func TestResultFindInsertionPoint_handlesNullObjects(t *testing.T) {
	t.Skip("Not yet implemented")
}

func TestResultFindObject(t *testing.T) {
	// create an object we want to extract
	source := map[string]interface{}{
		"hello": []interface{}{
			map[string]interface{}{
				"firstName": "0",
				"friends": []interface{}{
					map[string]interface{}{
						"firstName": "2",
						"friends": []interface{}{
							map[string]interface{}{
								"firstName": "Hello1",
							},
						},
					},
					map[string]interface{}{
						"firstName": "3",
						"friends": []interface{}{
							map[string]interface{}{
								"firstName": "Hello2",
							},
						},
					},
				},
			},
			map[string]interface{}{
				"firstName": "4",
				"friends": []interface{}{
					map[string]interface{}{
						"firstName": "5",
						"friends": []interface{}{
							map[string]interface{}{
								"firstName": "Hello3",
							},
						},
					},
					map[string]interface{}{
						"firstName": "6",
						"friends": []interface{}{
							map[string]interface{}{
								"firstName": "Hello4",
							},
						},
					},
				},
			},
		},
	}

	cachedPointDataExtractor := &CachedPointDataExtractor{cache: map[string]*PointData{}}
	value, err := ExtractValueModifyingSource(
		cachedPointDataExtractor,
		source,
		[]string{"hello:0", "friends:1", "friends:0"},
	)
	assert.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"firstName": "Hello2",
	}, value)
}

func TestResultSingleObjectWithColonInID(t *testing.T) {
	var source = make(map[string]interface{})
	_ = json.Unmarshal([]byte(
		// language=JSON
		`{"hello": {"id": "Thing:1337", "firstName": "Foo", "lastName": "bar"}}`),
		&source,
	)

	cachedPointDataExtractor := &CachedPointDataExtractor{cache: map[string]*PointData{}}
	value, err := ExtractValueModifyingSource(
		cachedPointDataExtractor,
		source,
		[]string{"hello#Thing:1337"},
	)
	assert.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"id": "Thing:1337", "firstName": "Foo", "lastName": "bar",
	}, value)
}

func TestResultSingleObjectModifyingStructure(t *testing.T) {
	var source = make(map[string]interface{})
	_ = json.Unmarshal([]byte(
		// language=JSON
		`{"hello": {}`),
		&source,
	)

	cachedPointDataExtractor := &CachedPointDataExtractor{cache: map[string]*PointData{}}
	value, err := ExtractValueModifyingSource(
		cachedPointDataExtractor,
		source,
		[]string{"hello#Thing:1337"},
	)
	assert.NoError(t, err)

	assert.Equal(t, map[string]interface{}{}, value)
	assert.Equal(t, map[string]interface{}{"hello": map[string]interface{}{}}, source)
}

func TestResultExtractID(t *testing.T) {
	type Case struct {
		Obj   map[string]interface{}
		Res   interface{}
		IsErr bool
	}

	for _, c := range []Case{{
		Obj:   map[string]interface{}{"id": "1"},
		Res:   "1",
		IsErr: false,
	}, {
		Obj:   map[string]interface{}{"id": 1},
		Res:   1,
		IsErr: false,
	}, {
		Obj:   map[string]interface{}{"id": "1", "__typename": "Object"},
		Res:   "1",
		IsErr: false,
	}, {
		Obj:   map[string]interface{}{"__typename": "Object"},
		Res:   nil,
		IsErr: false,
	}, {
		Obj:   map[string]interface{}{"__typename": "Object", "other": "1"},
		Res:   nil,
		IsErr: true,
	}, {
		Obj:   map[string]interface{}{},
		Res:   nil,
		IsErr: true,
	}, {
		Obj:   map[string]interface{}{"other": "1"},
		Res:   nil,
		IsErr: true,
	}} {
		actual, err := extractID(c.Obj)

		if c.IsErr {
			assert.Error(t, err)
			continue
		}

		if c.Res == nil {
			assert.Nil(t, actual)
			continue
		}

		assert.Equal(t, c.Res, actual)
	}
}
