package planner

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/buildbuildio/pebbles/merger"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func deepSortPlanSteps(qps []*QueryPlanStep) {
	sort.SliceStable(qps, func(i, j int) bool {
		return qps[i].URL+qps[i].QueryString < qps[j].URL+qps[j].QueryString
	})
	for _, step := range qps {
		if step.Then != nil {
			deepSortPlanSteps(step.Then)
		}
	}
}

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

	if actual != nil && actual.RootSteps != nil {
		deepSortPlanSteps(actual.RootSteps)
	}

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

	type Zoo {
		name: String!
		animals: [Animal!]!
	}

	type Query {
		getAnimals: [Animal!]!
		getZoos: [Zoo!]!
		node(id: ID!): Node
	}
`

var unionTum = merger.TypeURLMap{
	"Query": {
		Fields: map[string]string{
			"getAnimals": "0",
			"getZoos":    "0",
		},
	},
	"Zoo": {
		Fields: map[string]string{
			"name":    "0",
			"animals": "0",
		},
		IsImplementsNode: false,
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

var deepSchema = `
	interface Node {
		id: ID!
	}

	type Animal implements Node {
		id: ID!
		name: String!
		species: Species!
	}

	type Species implements Node {
		id: ID!
		name: String!
		genus: Genus!
	}

	type Genus implements Node {
		id: ID!
		name: String!
	}

	type Query {
		getAnimals: [Animal!]!
		node(id: ID!): Node
	}
`

var deepTum = merger.TypeURLMap{
	"Query": {
		Fields: map[string]string{
			"getAnimals": "0",
		},
	},
	"Animal": {
		Fields: map[string]string{
			"name":    "0",
			"species": "0",
		},
		IsImplementsNode: true,
	},
	"Species": {
		Fields: map[string]string{
			"name":  "1",
			"genus": "1",
		},
		IsImplementsNode: true,
	},
	"Genus": {
		Fields: map[string]string{
			"name": "1",
		},
		IsImplementsNode: true,
	},
}

var extendSchema = `
	interface Node {
		id: ID!
	}

	type User implements Node {
		id: ID!
		name: String!
		ownedCompany: Company!
		workingAs: Employee!
		
	}

	type Company implements Node {
		id: ID!
		name: String!
		employees: [Employee!]!
	}

	type Employee implements Node {
		id: ID!
		user: User!
		name: String!
	}

	type Query {
		getUsers: [User!]!
		node(id: ID!): Node
	}
`

var extendTum = merger.TypeURLMap{
	"Query": {
		Fields: map[string]string{
			"getUsers": "0",
		},
	},
	"User": {
		Fields: map[string]string{
			"name":         "0",
			"ownedCompany": "1",
			"workingAs":    "1",
		},
		IsImplementsNode: true,
	},
	"Company": {
		Fields: map[string]string{
			"name":      "1",
			"employees": "1",
		},
		IsImplementsNode: true,
	},
	"Employee": {
		Fields: map[string]string{
			"name": "1",
			"user": "1",
		},
		IsImplementsNode: true,
	},
}
