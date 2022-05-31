package planner

import (
	"encoding/json"
	"testing"

	"github.com/buildbuildio/pebbles/merger"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func mustRunPlanner(t *testing.T, p Planner, schema, query string, tum merger.TypeURLMap) (string, *QueryPlan) {
	t.Helper()

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})

	operation := gqlparser.MustLoadQuery(s, query)
	require.Len(t, operation.Operations, 1, "bad test: query must be a single operation")

	actual, err := p.Plan(&PlanningContext{
		Operation:  operation.Operations[0],
		Schema:     s,
		TypeURLMap: tum,
	})
	require.NoError(t, err)
	v, _ := json.Marshal(actual)
	return string(v), actual
}

var simpleSchema = `
	interface Node {
		id: ID!
	}

	enum Language {
		French
		English
		Italian
	}

	type Movie implements Node {
		id: ID!
		title(language: Language): String!
		author: Author!
		filmedBy: Author!
	}

	type Author implements Node {
		id: ID!
		name: String!
		movies: [Movie!]!
	}

	input AuthorInput {
		name: String!
	}

	type Query {
		getMovies: [Movie!]!
		getAuthors: [Author!]!
		node(id: ID!): Node
	}

	type Mutation {
		saveAuthor(input: AuthorInput!): Author!
	}
`

var simpleTum = merger.TypeURLMap{
	"Query": {
		Fields: map[string]string{
			"getMovies":  "0",
			"getAuthors": "1",
		},
	},
	"Mutation": {
		Fields: map[string]string{
			"saveAuthor": "1",
		},
	},
	"Movie": {
		Fields: map[string]string{
			"title":    "0",
			"author":   "0",
			"filmedBy": "0",
		},
		IsImplementsNode: true,
	},
	"Author": {
		Fields: map[string]string{
			"name":   "1",
			"movies": "0",
		},
		IsImplementsNode: true,
	},
}

var unionSchema = `
	interface Node {
		id: ID!
	}

	union Animal = Cat | Dog | Wolf

	type Cat implements Node {
		id: ID!
		name: String!
	}

	type Dog implements Node {
		id: ID!
		name: String!
		trained: Boolean!
	}

	type Wolf implements Node {
		id: ID!
		species: String!
	}

	type Query {
		getAnimals: [Animal!]!
		node(id: ID!): Node
	}
`

var unionTum = merger.TypeURLMap{
	"Query": {
		Fields: map[string]string{
			"getAnimals": "0",
		},
	},
	"Dog": {
		Fields: map[string]string{
			"name":    "1",
			"trained": "1",
		},
		IsImplementsNode: true,
	},
	"Cat": {
		Fields: map[string]string{
			"name": "1",
		},
		IsImplementsNode: true,
	},
	"Wolf": {
		Fields: map[string]string{
			"species": "0",
		},
		IsImplementsNode: true,
	},
}

var interfaceSchema = `
	interface Node {
		id: ID!
	}

	interface User {
		id: ID!
		name: String!
		files: [File!]!
	}

	type BasicUser implements User & Node {
		id: ID!
		name: String!
		files: [File!]!
	}

	type OtherUser implements User & Node {
		id: ID!
		name: String!
		phone: String!
		files: [File!]!
	}

	type File implements Node {
		id: ID!
		name: String!
		creator: User!
		dims: [Dim!]!
	}

	type Dim implements Node {
		id: ID!
		resolution: Int!
		file: File!
	}

	type Query {
		getUsers: [User!]!
		getFiles: [File!]!
		node(id: ID!): Node
}
`

var interfaceTum = merger.TypeURLMap{
	"Query": {
		Fields: map[string]string{
			"getUsers": "0",
			"getFiles": "1",
		},
	},
	"File": {
		Fields: map[string]string{
			"name":    "1",
			"creator": "1",
			"dims":    "2",
		},
		IsImplementsNode: true,
	},
	"Dim": {
		Fields: map[string]string{
			"resolution": "2",
			"file":       "2",
		},
		IsImplementsNode: true,
	},
	"BasicUser": {
		Fields: map[string]string{
			"name":  "0",
			"files": "1",
		},
		IsImplementsNode: true,
	},
	"OtherUser": {
		Fields: map[string]string{
			"name":  "0",
			"phone": "0",
			"files": "1",
		},
		IsImplementsNode: true,
	},
}
