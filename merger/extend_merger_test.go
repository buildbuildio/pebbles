package merger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

var extendMerger ExtendMergerFunc

func TestBadSchemas(t *testing.T) {
	for _, v := range [][]string{nil, {}, {"bad schema"}} {
		assert.Panics(t, func() {
			mustRunMerger(t, extendMerger, v)
		})
	}
}

func TestMergeSingleSchema(t *testing.T) {
	schemas := []string{
		`
		interface Node {
			id: ID!
		}

		input HumanInput {
			name: String!
		}

		type Human implements Node {
			id: ID!
			name: String!
		}

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}

		type Mutation {
			saveHuman(input: HumanInput!): Human!
		}
		`,
	}

	res, tm := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, schemas[0], res)

	assert.JSONEq(t, `
		{
			"Query": {
				"Fields": {"getHuman":  "0"},
				"IsImplementsNode": false
			},
			"Mutation": {
				"Fields": {"saveHuman": "0"},
				"IsImplementsNode": false
			},
			"Human": {
				"Fields": {"name": "0"},
				"IsImplementsNode": true
			}
		}
	`, tm)
}

func TestMergeTwoSchemas(t *testing.T) {
	schemas := []string{
		`
		interface Node {
			"Node description"
			id: ID!
		}

		"First description"
		type Human implements Node {
			id: ID!
			age: Int!
		}

		type Photo {
			url: String!
		}

		type Query {
			getPhotos: [Photo!]!
			node(id: ID!): Node
		}
		`,
		`
		directive @someDirective on FIELD_DEFINITION
		
		interface Node {
			id: ID!
		}

		interface Name {
			name: String!
		}

		"Second description"
		type Human implements Node {
			id: ID!
			name: String! @someDirective
		}

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}
		`,
	}
	expected := `
		directive @someDirective on FIELD_DEFINITION

		"""
		First description

		Second description
		"""
		type Human implements Node {
			id: ID!
			name: String! @someDirective
			age: Int!
		}

		interface Name {
			name: String!
		}

		interface Node {
			"""
			Node description
			"""
			id: ID!
		}

		type Photo {
			url: String!
		}

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
			getPhotos: [Photo!]!
		}
	`

	res, tm := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)

	assert.JSONEq(t, `
		{
			"Query": {
				"Fields": {"getPhotos": "0", "getHuman":  "1"},
				"IsImplementsNode": false
			},
			"Human": {
				"Fields": {"age":  "0", "name": "1"},
				"IsImplementsNode": true
			},
			"Photo": {
				"Fields": {"url": "0"},
				"IsImplementsNode": false
			}
		}
	`, tm)
}

func TestOnlyOneOfImplementingNodeInterface(t *testing.T) {
	schemas := []string{
		`
		interface Node { id: ID! }
		type Human implements Node { id: ID!, name: String! }

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}
		`,
		`
		interface Node { id: ID! }
		type Human { id: ID!, otherName: String! }

		type Query {
			node(id: ID!): Node
		}
		`,
	}

	assert.Panics(t, func() {
		mustRunMerger(t, extendMerger, schemas)
	})

}

func TestSameQuery(t *testing.T) {
	schemas := []string{
		`
		interface Node { id: ID! }
		type Human implements Node { id: ID!, name: String! }

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}
		`,
		`
		interface Node { id: ID! }
		type Human { id: ID!, otherName: String! }

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}
		`,
	}

	assert.Panics(t, func() {
		mustRunMerger(t, extendMerger, schemas)
	})
}

func TestSameMutation(t *testing.T) {
	schemas := []string{
		`
		interface Node { id: ID! }
		type Human implements Node { id: ID!, name: String! }

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}

		type Mutation {
			saveHuman(name: String!): Human!
		}
		`,
		`
		interface Node { id: ID! }
		type Human { id: ID!, otherName: String! }

		type Query {
			getHuman(id: ID!): Human!
			node(id: ID!): Node
		}

		type Mutation {
			saveHuman(otherName: String!): Human!
		}
		`,
	}

	assert.Panics(t, func() {
		mustRunMerger(t, extendMerger, schemas)
	})
}

func TestTypeImplementingNodeOverlappingFields(t *testing.T) {
	schemas := []string{
		`
			interface Node { id: ID! }
			type Human implements Node { id: ID!, name: String! }

			type Query {
				getHuman(id: ID!): Human!
			}
		`, `
			interface Node { id: ID! }
			type Human implements Node { id: ID!, name: String!, size: Float! }

			type Query {
				node(id: ID!): Node
			}
		`,
	}
	assert.PanicsWithError(t, "overlapping fields Human : name", func() {
		mustRunMerger(t, extendMerger, schemas)
	})
}

func TestTypeSameFields(t *testing.T) {
	schemas := []string{
		`
			type PaginateOptions { cursor: String!, limit: Int! }

			type Human {
				name: String!
			}

			type Query {
				getHuman: Human!
			}
		`, `
			type PaginateOptions { cursor: String!, limit: Int! }

			type Animal {
				name: String!
			}

			type Query {
				getAnimal: Animal!
			}
		`,
	}
	expected := `
		type Animal {
			name: String!
		}
		type Human {
			name: String!
		}
		type PaginateOptions {
			cursor: String!
			limit: Int!
		}
		type Query {
			getAnimal: Animal!
			getHuman: Human!
		}
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestTypeOverlappingFields(t *testing.T) {
	a := `
		type PaginateOptions { cursor: String! }

		type Human {
			name: String!
		}

		type Query {
			getHuman: Human!
		}
	`
	b := `
		type PaginateOptions { cursor: String!, limit: Int! }

		type Animal {
			name: String!
		}

		type Query {
			getAnimal: Animal!
		}
	`

	for _, schemas := range [][]string{{a, b}, {b, a}} {
		assert.PanicsWithError(t, "overlapping fields, not complete copy PaginateOptions : cursor", func() {
			mustRunMerger(t, extendMerger, schemas)
		})
	}

}

func TestMergeSupportsValidUnions(t *testing.T) {
	schemas := []string{
		`
			type Dog { name: String! }
			type Cat { name: String! }
			type Snake { name: String! }
			union Animal = Dog | Cat | Snake

			type Query {
				animals: [Animal]!
			}
		`,
		`
			type Circle { area: Float! }
			type Triangle { area: Float! }
			type Square { area: Float! }
			union Shape = Circle | Triangle | Square

			type Query {
				shapes: [Shape]!
			}
		`,
	}
	expected := `
		type Dog { name: String! }
		type Cat { name: String! }
		type Snake { name: String! }
		union Animal = Dog | Cat | Snake

		type Circle { area: Float! }
		type Triangle { area: Float! }
		type Square { area: Float! }
		union Shape = Circle | Triangle | Square

		type Query {
			shapes: [Shape]!
			animals: [Animal]!
		}
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestMergeSameEnums(t *testing.T) {
	schemas := []string{
		`
			directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
			enum CacheControlScope {
				PUBLIC
				PRIVATE
			}

			type Query {
				hello: String!
			}
		`,
		`
			directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
			enum CacheControlScope {
				PUBLIC
				PRIVATE
			}

			type Query {
				goodbye: String!
			}
		`,
	}
	expected := `
		directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
		enum CacheControlScope {
			PUBLIC
			PRIVATE
		}

		type Query {
			goodbye: String!
			hello: String!
		}
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestMergeExtendEnums(t *testing.T) {
	schemas := []string{
		`
			directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
			enum CacheControlScope {
				PUBLIC
				PRIVATE
				ANOTHER
			}

			type Query {
				hello: String!
			}
		`,
		`
			directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
			enum CacheControlScope {
				PUBLIC
				PRIVATE
				ANOTHER
				OTHER
			}

			type Query {
				goodbye: String!
			}
		`,
	}
	expected := `
		directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
		enum CacheControlScope {
			PUBLIC
			PRIVATE
			ANOTHER
			OTHER
		}

		type Query {
			goodbye: String!
			hello: String!
		}
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestMergeExtendDirectives(t *testing.T) {
	schemas := []string{
		`
			directive @cacheControl on FIELD_DEFINITION
			enum CacheControlScope {
				PUBLIC
				PRIVATE
				ANOTHER
			}

			type Query {
				hello: String!
			}
		`,
		`
			directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
			enum CacheControlScope {
				PUBLIC
				PRIVATE
				ANOTHER
				OTHER
			}

			type Query {
				goodbye: String!
			}
		`,
	}
	expected := `
		directive @cacheControl on FIELD_DEFINITION | OBJECT | INTERFACE
		enum CacheControlScope {
			PUBLIC
			PRIVATE
			ANOTHER
			OTHER
		}

		type Query {
			goodbye: String!
			hello: String!
		}
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestMergeHandlesUnionConflict(t *testing.T) {
	schemas := []string{
		`
			type Dog1 { name: String! }
			type Cat1 { name: String! }
			type Snake1 { name: String! }
			union Animal = Dog1 | Cat1 | Snake1

			type Query {
				animals: [Animal]!
			}
		`,
		`
			type Dog2 { name: String! }
			type Cat2 { name: String! }
			type Snake2 { name: String! }
			union Animal = Dog2 | Cat2 | Snake2

			type Query {
				foo: String!
			}
		`,
	}

	assert.PanicsWithError(t, "union collision: Animal(UNION) conflicting types [Dog1 Cat1 Snake1]([Dog2 Cat2 Snake2])", func() {
		mustRunMerger(t, extendMerger, schemas)
	})
}

func TestMergeSupportsSpreadUnions(t *testing.T) {
	schemas := []string{
		`
			interface Node { id: ID! }
			type Dog implements Node { id: ID!, name: String! }
			type Cat implements Node { id: ID!, name: String! }
			type Snake implements Node { id: ID!, name: String! }
			union Animal = Dog | Cat | Snake

			type Query {
				node(id: ID!): Node
				animals: [Animal]!
			}
		`,
		`
			interface Node { id: ID! }
			type Dog implements Node { id: ID!, age: Int! }
			type Cat implements Node { id: ID!, age: Int! }
			type Snake implements Node { id: ID!, age: Int! }
			union Animal = Dog | Cat | Snake

			type Query {
				node(id: ID!): Node
			}
		`,
	}
	expected := `
		interface Node { id: ID! }
		type Cat implements Node { id: ID!, age: Int!, name: String! }
		type Dog implements Node { id: ID!, age: Int!, name: String! }
		type Snake implements Node { id: ID!, age: Int!, name: String! }
		union Animal = Dog | Cat | Snake

		type Query {
			node(id: ID!): Node
			animals: [Animal]!
		}
		
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestMergeSupportsSpreadInterfaces(t *testing.T) {
	schemas := []string{
		`
			interface Node {
				id: ID!
			}
			
			interface User {
				id: ID!
				name: String!
			}
			
			type BasicUser implements User & Node {
				id: ID!
				name: String!
			}
			
			type OtherUser implements User & Node {
				id: ID!
				name: String!
				phone: String!
			}
			
			type Query {
				getUsers: [User!]!
				node(id: ID!): Node
			}
		`,
		`
			interface Node {
				id: ID!
			}
			
			interface User {
				id: ID!
				files: [File!]!
			}
			
			type BasicUser implements User & Node {
				id: ID!
				files: [File!]!
			}
			
			type OtherUser implements User & Node {
				id: ID!
				files: [File!]!
			}
			
			type File implements Node {
				id: ID!
				name: String!
				creator: User!
			}
			
			type Query {
				node(id: ID!): Node
			}
		`,
	}

	expected := `
		interface Node {
			id: ID!
		}
		
		interface User {
			id: ID!
			files: [File!]!
			name: String!
		}
		
		type BasicUser implements User & Node {
			id: ID!
			files: [File!]!
			name: String!
		}
		
		type OtherUser implements User & Node {
			id: ID!
			files: [File!]!
			name: String!
			phone: String!
		}
		
		type File implements Node {
			id: ID!
			name: String!
			creator: User!
		}
		
		type Query {
			node(id: ID!): Node
			getUsers: [User!]!
		}
	`

	res, tm := mustRunMerger(t, extendMerger, schemas)

	assert.JSONEq(t, `
		{
			"Query": {
				"Fields": {"getUsers": "0"},
				"IsImplementsNode": false
			},
			"File": {
				"Fields": {"name": "1", "creator": "1"},
				"IsImplementsNode": true
			},
			"BasicUser": {
				"Fields": {"name": "0", "files": "1"},
				"IsImplementsNode": true
			},
			"OtherUser": {
				"Fields": {"name": "0", "phone": "0", "files": "1"},
				"IsImplementsNode": true
			}
		}
	`, tm)

	isEqualSchemas(t, expected, res)
}

func TestOverlapingMutations(t *testing.T) {
	schemas := []string{
		`
			type Mutation {
				saveHuman(name: String!): ID!
			}
		`, `
			type Mutation {
				saveHuman(name: String!): ID!
			}
		`,
	}
	assert.PanicsWithError(t, "overlapping root types fields Mutation : saveHuman", func() {
		mustRunMerger(t, extendMerger, schemas)
	})
}

func TestMergeCustomnScalars(t *testing.T) {
	schemas := []string{
		`
			scalar Test
		`,
		`
			scalar Test
		`,
	}
	expected := `
		scalar Test
	`

	res, _ := mustRunMerger(t, extendMerger, schemas)

	isEqualSchemas(t, expected, res)
}

func TestMergeMissingPossibleTypes(t *testing.T) {
	inps := []*MergeInput{{
		Schema: &ast.Schema{
			Types: map[string]*ast.Definition{
				"Union": {
					Kind:  ast.Union,
					Name:  "Union",
					Types: nil, // checking this line
				},
				"A": {
					Kind: ast.Object,
					Name: "A",
					Fields: ast.FieldList{{
						Name: "f",
						Type: ast.NamedType("String", nil),
					}},
				},
			},
			PossibleTypes: map[string][]*ast.Definition{
				"Union": {{
					Kind: "String",
					Name: "A",
				}},
			},
		},
		URL: "0",
	}}
	expected := `
		union Union = A

		type A {
			f: String
		}
	`

	res, err := extendMerger.Merge(inps)
	assert.NoError(t, err)

	s := formatSchema(res.Schema)

	isEqualSchemas(t, expected, s)
}
