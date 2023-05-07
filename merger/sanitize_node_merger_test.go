package merger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var sanitizeMerger SanitizeNodeMergerFunc

func TestMergeSingleSchemaSanitized(t *testing.T) {
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

	resSchema := `
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
		}

		type Mutation {
			saveHuman(input: HumanInput!): Human!
		}
	`

	res, tm := mustRunMerger(t, sanitizeMerger, schemas)

	isEqualSchemas(t, resSchema, res)

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
