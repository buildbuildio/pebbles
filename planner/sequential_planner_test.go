package planner

import (
	"fmt"
	"testing"

	"github.com/buildbuildio/pebbles/common"

	"github.com/stretchr/testify/assert"
)

var seqPlan SequentialPlanner

func TestSimplePlan(t *testing.T) {
	query := `{ getMovies { id title(language: French) author { id name } }}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { id title(language: French) author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanNode(t *testing.T) {
	query := `{ node(id: "author_2556") {... on Author {id}}}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ node(id: \"author_2556\") { ... on Author { id } } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": {"node#Author": ["__typename"], "node#Movie": ["__typename"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanNodeManyFields(t *testing.T) {
	query := `{ node(id: "author_2556") {... on Author {id name movies {id}}}}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ node(id: \"author_2556\") { ... on Author { movies { id } } id } }",
			"InsertionPoint": null,
			"Then": null
		  },
		  {
			"URL": "1",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ node(id: \"author_2556\") { ... on Author { name } id } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": {"node#Author": ["__typename"], "node#Movie": ["__typename"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanOperationName(t *testing.T) {
	query := `query Operation { getMovies { id title(language: French) author { id name } }}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": "Operation",
			"SelectionSet": "{ getMovies { id title(language: French) author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanWithTypename(t *testing.T) {
	query := `{ getMovies { __typename id title(language: French) author { __typename id name } }}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { __typename id title(language: French) author { __typename id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimpleMutation(t *testing.T) {
	query := `mutation { saveAuthor(input: {name: "Name"}) {id name movies { id title(language: French) }}}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "1",
			"ParentType": "Mutation",
			"OperationName": null,
			"SelectionSet": "{ saveAuthor(input: {name:\"Name\"}) { id name } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "0",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { movies { id title(language: French) } } } }",
					"InsertionPoint": ["saveAuthor"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimpleSubscription(t *testing.T) {
	query := `subscription { authorAdded {id name movies { id title(language: French) }}}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "1",
			"ParentType": "Subscription",
			"OperationName": null,
			"SelectionSet": "{ authorAdded { id name } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "0",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { movies { id title(language: French) } } } }",
					"InsertionPoint": ["authorAdded"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanMultipleQueries(t *testing.T) {
	query := `{ a: getMovies { id title(language: French) author { id name } } b: getMovies { id title(language: French) author { id name } }}`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ a: getMovies { id title(language: French) author { id } } b: getMovies { id title(language: French) author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["a", "author"],
					"Then": null
				},
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["b", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanNoIDs(t *testing.T) {
	query := `{ getMovies { title(language: French) author { name } } }`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { id title(language: French) author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": {"getMovies.author#Author": ["id"], "getMovies#Movie": ["id"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanNoIDsAlias(t *testing.T) {
	query := `{ m: getMovies { t: title(language: French) a: author { n: name } } }`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ m: getMovies { id t: title(language: French) a: author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { n: name } } }",
					"InsertionPoint": ["m", "a"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": {"m.a#Author": ["id"], "m#Movie": ["id"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanNoIDsDeep(t *testing.T) {
	query := `{ getMovies { author { name movies { title(language: French) filmedBy { name } } } } }`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { id author { id movies { id title(language: French) filmedBy { id } } } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				},
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author", "movies", "filmedBy"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": {
			"getMovies#Movie": ["id"],
			"getMovies.author#Author": ["id"],
			"getMovies.author.movies#Movie": ["id"],
			"getMovies.author.movies.filmedBy#Author": ["id"]
		}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanInlineFragment(t *testing.T) {
	query := `{ getMovies { __typename ... on Movie { id, title(language: French) author { id name } } } }`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { __typename id title(language: French) author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanSpreadFragment(t *testing.T) {
	query := `
	fragment Frag on Movie {
		__typename
		id
		title(language: French)
		author { 
			id 
			name
		}
	}
	{
		getMovies {
			...Frag
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { __typename id title(language: French) author { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSimplePlanSpreadFragmentManyUsages(t *testing.T) {
	query := `
	fragment Frag on Author {
		id
		name
	}
	{
		getMovies {
			id
			author { ...Frag }
			filmedBy { ...Frag }
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getMovies { id author { id } filmedBy { id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "author"],
					"Then": null
				},
				{
					"URL": "1",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Author { name } } }",
					"InsertionPoint": ["getMovies", "filmedBy"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestSameTypeDifferentURLsPlan(t *testing.T) {
	query := `{ getHumans { name }}`

	actual, _ := mustRunPlanner(t, seqPlan, sameTypeDifferentURLsSchema, query, sameTypeDifferentURLsTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getHumans { name } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestDeepPlan(t *testing.T) {
	query := `{ getAnimals { __typename id name species {__typename id name genus {__typename id name}} }}`

	actual, plan := mustRunPlanner(t, seqPlan, deepSchema, query, deepTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getAnimals { __typename id name species { __typename id } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Species",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Species { name genus { __typename id name } } } }",
					"InsertionPoint": ["getAnimals", "species"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.Equal(
		t,
		"query ($id: ID!) {\n\tnode(id: $id) {\n\t\t... on Species {\n\t\t\tname\n\t\t\tgenus {\n\t\t\t\t__typename\n\t\t\t\tid\n\t\t\t\tname\n\t\t\t}\n\t\t}\n\t}\n}",
		plan.RootSteps[0].Then[0].QueryString,
	)

	assert.JSONEq(t, expected, actual)
}

func TestExtendPlan(t *testing.T) {
	query := `{ getUsers { __typename id name ownedCompany { __typename id name } workingAs { __typename id user { __typename id name } } } }`

	actual, _ := mustRunPlanner(t, seqPlan, extendSchema, query, extendTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getUsers { __typename id name } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "User",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on User { ownedCompany { __typename id name } workingAs { __typename id user { __typename id } } } } }",
					"InsertionPoint": ["getUsers"],
					"Then": [{
						"URL": "0",
						"ParentType": "User",
						"OperationName": null,
						"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on User { name } } }",
						"InsertionPoint": ["getUsers", "workingAs", "user"],
						"Then": null
					}]
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestUnionPlanUnion(t *testing.T) {
	for _, query := range []string{`
	{
		getAnimals {
			__typename
			... on Cat {
				id
				name
			}
			... on Dog {
				id
				name
				trained
			}
			... on Wolf {
				id
				species
			}
		}
	}
	`, `
	{
		getAnimals {
			__typename
			...Animal
		}
	}

	fragment Animal on Animal {
		__typename
		... on Cat {
			...Cat
		}
		... on Dog {
			...Dog
		}
		... on Wolf {
			...Wolf
		}
	}

	fragment Cat on Cat {
		id
		name
	}

	fragment Dog on Dog {
		id
		name
		trained
	}

	fragment Wolf on Wolf {
		id
		species
	}
	`} {
		actual, _ := mustRunPlanner(t, seqPlan, unionSchema, query, unionTum)

		expected := `{
			"RootSteps": [
			  {
				"URL": "0",
				"ParentType": "Query",
				"OperationName": null,
				"SelectionSet": "{ getAnimals { __typename ... on Cat { id } ... on Dog { id } ... on Wolf { id species } } }",
				"InsertionPoint": null,
				"Then": [
					{
						"URL": "1",
						"ParentType": "Cat",
						"OperationName": null,
						"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Cat { name } } }",
						"InsertionPoint": ["getAnimals"],
						"Then": null
					},
					{
						"URL": "1",
						"ParentType": "Dog",
						"OperationName": null,
						"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Dog { name trained } } }",
						"InsertionPoint": ["getAnimals"],
						"Then": null
					}
				]
			  }
			],
			"ScrubFields": null
		  }`

		assert.JSONEq(t, expected, actual)
	}
}

func TestUnionPlanJustTypename(t *testing.T) {
	query := `
	{
		getAnimals {
			__typename
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, unionSchema, query, unionTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getAnimals { __typename } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestUnionPlanUnionPartialScrubFields(t *testing.T) {
	query := `
	{
		getAnimals {
			... on Cat {
				name
			}
			... on Dog {
				id
				name
				trained
			}
			... on Wolf {
				species
			}
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, unionSchema, query, unionTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getAnimals { __typename ... on Cat { id } ... on Dog { id } ... on Wolf { id species } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "Cat",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Cat { name } } }",
					"InsertionPoint": ["getAnimals"],
					"Then": null
				},
				{
					"URL": "1",
					"ParentType": "Dog",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on Dog { name trained } } }",
					"InsertionPoint": ["getAnimals"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": {"getAnimals#Cat": ["id", "__typename"], "getAnimals#Wolf": ["id", "__typename"], "getAnimals#Dog": ["__typename"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestUnionSingleSchemaSpread(t *testing.T) {
	query := `
	{
		getZoos {
			__typename
			name
			animals {
				__typename
				... on Wolf {
					...AnimalFragment
				}
			}
		}
	}

	fragment AnimalFragment on Animal {
		... on Wolf {
			...Wolf
		}
		__typename
	}

	fragment Wolf on Wolf {
		__typename
		id
		species
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, unionSchema, query, unionTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getZoos { __typename name animals { __typename ... on Wolf { __typename id species } } } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestInterfacePlanInlineSimple(t *testing.T) {
	query := `
	{
		getUsers {
			id
			name
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, interfaceSchema, query, interfaceTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getUsers { __typename id name } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": {"getUsers#BasicUser": ["__typename"], "getUsers#OtherUser": ["__typename"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestInterfacePlanInlineComplex(t *testing.T) {
	for _, query := range []string{`
		{
			getUsers {
				__typename
				id
				name
				files {
					id
				}
			}
		}
	`, `
		fragment Frag on User {
			__typename
			id
			name
			files {
				id
			}
		}
		{
			getUsers {
				...Frag
			}
		}
	`, `
		{
			getUsers {
				__typename
				... on BasicUser {
					id
					name
					files {
						id
					}
				}
				... on OtherUser {
					id
					name
					files {
						id
					}
				}
			}
		}
	`} {
		actual, _ := mustRunPlanner(t, seqPlan, interfaceSchema, query, interfaceTum)

		expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getUsers { __typename ... on BasicUser { id name } ... on OtherUser { id name } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "BasicUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on BasicUser { files { id } } } }",
					"InsertionPoint": ["getUsers"],
					"Then": null
				},
				{
					"URL": "1",
					"ParentType": "OtherUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on OtherUser { files { id } } } }",
					"InsertionPoint": ["getUsers"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

		assert.JSONEq(t, expected, actual, query)
	}

}

func TestInterfacePlanInlineComplexDifferentFields(t *testing.T) {
	query := `
	{
		getUsers {
			__typename
			... on BasicUser {
				id
				name
				files {
					id
					name
				}
			}
			... on OtherUser {
				name
				phone
			}
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, interfaceSchema, query, interfaceTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getUsers { __typename ... on BasicUser { id name } ... on OtherUser { id name phone } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "BasicUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on BasicUser { files { id name } } } }",
					"InsertionPoint": ["getUsers"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": {"getUsers#OtherUser": ["id"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestInterfacePlanInlineComplexOtherWay(t *testing.T) {
	query := `
	{
		getFiles {
			id
			name
			creator {
				__typename
				... on BasicUser {
					id
					name
					files {
						id
						name
					}
				}
				... on OtherUser {
					name
					phone
				}
			}
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, interfaceSchema, query, interfaceTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "1",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getFiles { id name creator { __typename ... on BasicUser { id files { id name } } ... on OtherUser { id } } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "0",
					"ParentType": "BasicUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on BasicUser { name } } }",
					"InsertionPoint": ["getFiles", "creator"],
					"Then": null
				},
				{
					"URL": "0",
					"ParentType": "OtherUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on OtherUser { name phone } } }",
					"InsertionPoint": ["getFiles", "creator"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": {"getFiles.creator#OtherUser": ["id"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestInterfacePlanInlineComplexThreeServices(t *testing.T) {
	query := `
	{
		getUsers {
			__typename
			id
			name
			files {
				id
				dims {
					id
					resolution
					file {
						id
						creator {
							name
						}
					}
				}
			}
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, interfaceSchema, query, interfaceTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "0",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getUsers { __typename ... on BasicUser { id name } ... on OtherUser { id name } } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "1",
					"ParentType": "BasicUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on BasicUser { files { id } } } }",
					"InsertionPoint": ["getUsers"],
					"Then": [
						{
							"URL": "2",
							"ParentType": "File",
							"OperationName": null,
							"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on File { dims { id resolution file { id } } } } }",
							"InsertionPoint": ["getUsers", "files"],
							"Then": [
								{
									"URL": "1",
									"ParentType": "File",
									"OperationName": null,
									"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on File { creator { __typename ... on BasicUser { id } ... on OtherUser { id } } } } }",
									"InsertionPoint": ["getUsers", "files", "dims", "file"],
									"Then": [
										{
											"URL": "0",
											"ParentType": "BasicUser",
											"OperationName": null,
											"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on BasicUser { name } } }",
											"InsertionPoint": ["getUsers", "files", "dims", "file", "creator"],
											"Then": null
										},
										{
											"URL": "0",
											"ParentType": "OtherUser",
											"OperationName": null,
											"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on OtherUser { name } } }",
											"InsertionPoint": ["getUsers", "files", "dims", "file", "creator"],
											"Then": null
										}
									]
								}
							]
						}
					]
				},
				{
					"URL": "1",
					"ParentType": "OtherUser",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on OtherUser { files { id } } } }",
					"InsertionPoint": ["getUsers"],
					"Then": [
						{
							"URL": "2",
							"ParentType": "File",
							"OperationName": null,
							"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on File { dims { id resolution file { id } } } } }",
							"InsertionPoint": ["getUsers", "files"],
							"Then": [
								{
									"URL": "1",
									"ParentType": "File",
									"OperationName": null,
									"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on File { creator { __typename ... on BasicUser { id } ... on OtherUser { id } } } } }",
									"InsertionPoint": ["getUsers", "files", "dims", "file"],
									"Then": [
										{
											"URL": "0",
											"ParentType": "BasicUser",
											"OperationName": null,
											"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on BasicUser { name } } }",
											"InsertionPoint": ["getUsers", "files", "dims", "file", "creator"],
											"Then": null
										},
										{
											"URL": "0",
											"ParentType": "OtherUser",
											"OperationName": null,
											"SelectionSet": "query ($id: ID!) { node(id: $id) { ... on OtherUser { name } } }",
											"InsertionPoint": ["getUsers", "files", "dims", "file", "creator"],
											"Then": null
										}
									]
								}
							]
						}
					]
				}
			]
		  }
		],
		"ScrubFields": {"getUsers.files.dims.file.creator#BasicUser": ["__typename", "id"], "getUsers.files.dims.file.creator#OtherUser": ["__typename", "id"]}
	  }`

	assert.JSONEq(t, expected, actual)
}

func TestIntrospection(t *testing.T) {
	query := `{
		__schema {
			queryType {
				name
			}
			mutationType {
				name
			}
			subscriptionType {
				name
			}
			types {
				...FullType
			}
			directives {
				name
				description
				locations
				args {
					...InputValue
				}
			}
		}
	}

	fragment FullType on __Type {
		kind
		name
		description
		fields(includeDeprecated: true) {
			name
			description
			args {
				...InputValue
			}
			type {
				...TypeRef
			}
			isDeprecated
			deprecationReason
		}
		inputFields {
			...InputValue
		}
		interfaces {
			...TypeRef
		}
		enumValues(includeDeprecated: true) {
			name
			description
			isDeprecated
			deprecationReason
		}
		possibleTypes {
			...TypeRef
		}
	}

	fragment InputValue on __InputValue {
		name
		description
		type {
			...TypeRef
		}
		defaultValue
	}

	fragment TypeRef on __Type {
		kind
		name
		ofType {
			kind
			name
			ofType {
				kind
				name
				ofType {
					kind
					name
					ofType {
						kind
						name
						ofType {
							kind
							name
							ofType {
								kind
								name
								ofType {
									kind
									name
								}
							}
						}
					}
				}
			}
		}
	}
	`

	actual, _ := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := fmt.Sprintf(`{
		"RootSteps": [
		  {
			"URL": "%s",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ __schema { queryType { name } mutationType { name } subscriptionType { name } types { kind name description fields(includeDeprecated: true) { name description args { name description type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } } defaultValue } type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } } isDeprecated deprecationReason } inputFields { name description type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } } defaultValue } interfaces { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } } enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason } possibleTypes { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } } } directives { name description locations args { name description type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } } defaultValue } } } }",
			"InsertionPoint": null,
			"Then": null
		  }
		],
		"ScrubFields": null
	  }`, common.InternalServiceName)

	assert.JSONEq(t, expected, actual)
}

func TestPlanInnerArguments(t *testing.T) {
	query := `query ($lang: Language!) { getAuthors { id movies {id title(language: $lang) } }}`

	actual, plan := mustRunPlanner(t, seqPlan, simpleSchema, query, simpleTum)

	expected := `{
		"RootSteps": [
		  {
			"URL": "1",
			"ParentType": "Query",
			"OperationName": null,
			"SelectionSet": "{ getAuthors { id } }",
			"InsertionPoint": null,
			"Then": [
				{
					"URL": "0",
					"ParentType": "Author",
					"OperationName": null,
					"SelectionSet": "query ($id: ID!, $lang: Language) { node(id: $id) { ... on Author { movies { id title(language: $lang) } } } }",
					"InsertionPoint": ["getAuthors"],
					"Then": null
				}
			]
		  }
		],
		"ScrubFields": null
	  }`

	assert.JSONEq(t, expected, actual)

	assert.EqualValues(t, "{\n\tgetAuthors {\n\t\tid\n\t}\n}", plan.RootSteps[0].QueryString)
	assert.Nil(t, plan.RootSteps[0].VariablesList)

	assert.EqualValues(
		t,
		"query ($id: ID!, $lang: Language) {\n\tnode(id: $id) {\n\t\t... on Author {\n\t\t\tmovies {\n\t\t\t\tid\n\t\t\t\ttitle(language: $lang)\n\t\t\t}\n\t\t}\n\t}\n}",
		plan.RootSteps[0].Then[0].QueryString,
	)
	assert.Equal(t, []string{"id", "lang"}, plan.RootSteps[0].Then[0].VariablesList)

}
