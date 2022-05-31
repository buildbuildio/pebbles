package introspection

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func checkSuccess(schema *ast.Schema, ir *IntrospectionResolver) func(t *testing.T, query string, expected string) {
	return func(t *testing.T, query string, expected string) {
		q := gqlparser.MustLoadQuery(schema, query)

		res := ir.ResolveIntrospectionFields(q.Operations[0].SelectionSet, schema)

		bres, err := json.Marshal(res)

		require.NoError(t, err)

		require.JSONEq(t, expected, string(bres))
	}
}

func TestIntrospection(t *testing.T) {
	schemaStr := `
	union MovieOrCinema = Movie | Cinema
	interface Person { name: String! }

	type Cast implements Person {
		name: String!
	}

	"""
	A bit like a film
	"""
	type Movie {
		id: ID!
		title: String @deprecated(reason: "Use something else")
		genres: [MovieGenre!]!
	}

	enum MovieGenre {
		ACTION
		COMEDY
		HORROR @deprecated(reason: "too scary")
		DRAMA
		ANIMATION
		ADVENTURE
		SCIENCE_FICTION
	}

	type Cinema {
		id: ID!
		name: String!
	}

	type Query {
		movie(id: ID!): Movie!
		movies: [Movie!]!
		somethingRandom: MovieOrCinema
		somePerson: Person
	}`

	schema := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schemaStr})

	ir := &IntrospectionResolver{
		Variables: nil,
	}

	validate := checkSuccess(schema, ir)

	t.Run("basic type fields", func(t *testing.T) {
		validate(t, `{
			__type(name: "Movie") {
				kind
				name
				description
			}
		}`, `
		{
			"__type": {
				"description": "A bit like a film",
				"kind": "OBJECT",
				"name": "Movie"
			}
		}
		`)
	})

	t.Run("basic aliased type fields", func(t *testing.T) {
		validate(t, `{
			movie: __type(name: "Movie") {
				type: kind
				n: name
				desc: description
			}
		}`, `
		{
			"movie": {
				"desc": "A bit like a film",
				"type": "OBJECT",
				"n": "Movie"
			}
		}
		`)
	})

	t.Run("lists and non-nulls", func(t *testing.T) {
		validate(t, `{
			__type(name: "Movie") {
				fields(includeDeprecated: true) {
					name
					isDeprecated
					deprecationReason
					type {
						name
						kind
						ofType {
							name
							kind
							ofType {
								name
								kind
								ofType {
									name
								}
							}
						}
					}
				}
			}
		}`, `
		{
			"__type": {
				"fields": [
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "id",
					"type": {
					"kind": "NON_NULL",
					"name": null,
					"ofType": {
						"kind": "SCALAR",
						"name": "ID",
						"ofType": null
					}
					}
				},
				{
					"deprecationReason": "Use something else",
					"isDeprecated": true,
					"name": "title",
					"type": {
					"kind": "SCALAR",
					"name": "String",
					"ofType": null
					}
				},
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "genres",
					"type": {
					"kind": "NON_NULL",
					"name": null,
					"ofType": {
						"kind": "LIST",
						"name": null,
						"ofType": {
						"kind": "NON_NULL",
						"name": null,
						"ofType": {
							"name": "MovieGenre"
						}
						}
					}
					}
				}
				]
			}
		}
		`)
	})

	t.Run("enum", func(t *testing.T) {
		validate(t, `{
			__type(name: "MovieGenre") {
				enumValues(includeDeprecated: true) {
					name
					isDeprecated
					deprecationReason
				}
			}
		}`, `
		{
			"__type": {
				"enumValues": [
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "ACTION"
				},
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "COMEDY"
				},
				{
					"deprecationReason": "too scary",
					"isDeprecated": true,
					"name": "HORROR"
				},
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "DRAMA"
				},
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "ANIMATION"
				},
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "ADVENTURE"
				},
				{
					"deprecationReason": null,
					"isDeprecated": false,
					"name": "SCIENCE_FICTION"
				}
				]
			}
			}
		`)
	})

	t.Run("union", func(t *testing.T) {
		validate(t, `{
			__type(name: "MovieOrCinema") {
				possibleTypes {
					name
				}
			}
		}`, `
		{
			"__type": {
				"possibleTypes": [
				{
					"name": "Movie"
				},
				{
					"name": "Cinema"
				}
				]
			}
			}
		`)
	})

	t.Run("type referenced only through an interface", func(t *testing.T) {
		validate(t, `{
			__type(name: "Cast") {
				kind
				name
			}
		}`, `
		{
			"__type": {
				"kind": "OBJECT",
				"name": "Cast"
			}
		}
		`)
	})

	t.Run("directive", func(t *testing.T) {
		// check persistant order
		for i := 0; i < 50; i++ {
			validate(t, `{
				__schema {
					directives {
						name
						args {
							name
							type {
								name
							}
						}
					}
				}
			}`, `
			{
				"__schema": {
				  "directives": [
					{
						"name": "deprecated",
						"args": [{
							"name": "reason",
							"type": {
								"name": "String"
							}
						}]
					},
					{
						"name": "include",
						"args": [{
							"name": "if",
							"type": {
								"name": null
							}
						}]
					},
					{
						"name": "skip",
						"args": [{
							"name": "if",
							"type": {
								"name": null
							}
						}]
					},
					{
						"name": "specifiedBy",
						"args": [{
							"name": "url",
							"type": {
								"name": null
							}
						}]
					}
				  ]
				}
			  }
			`)
		}

	})

	t.Run("__schema", func(t *testing.T) {
		validate(t, `{
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
			}
		}`, `
		{
			"__schema": {
				"queryType": {
					"name": "Query"
				},
				"mutationType": null,
				"subscriptionType": null
			}
		}
		`)
	})

	t.Run("__schema types", func(t *testing.T) {
		// check persistant order
		for i := 0; i < 50; i++ {
			validate(t, `{
				__schema {
					types {
						name
					}
				}
			}`, `
			{
				"__schema": {
					"types": [
						{"name":"Boolean"},
						{"name":"Cast"},
						{"name":"Cinema"},
						{"name":"Float"},
						{"name":"ID"},
						{"name":"Int"},
						{"name":"Movie"},
						{"name":"MovieGenre"},
						{"name":"MovieOrCinema"},
						{"name":"Person"},
						{"name":"Query"},
						{"name":"String"},
						{"name":"__Directive"},
						{"name":"__DirectiveLocation"},
						{"name":"__EnumValue"},
						{"name":"__Field"},
						{"name":"__InputValue"},
						{"name":"__Schema"},
						{"name":"__Type"},
						{"name":"__TypeKind"}
					]
				}
			}
			`)
		}
	})
}
