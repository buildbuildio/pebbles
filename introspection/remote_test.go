package introspection

import (
	"testing"

	"github.com/buildbuildio/pebbles/queryer"
	"github.com/stretchr/testify/assert"
)

func TestIntrospectRemoteSchemasMultiple(t *testing.T) {
	q1 := &MockSuccessQueryer{
		Value: `{ 
			"__schema": {
				"queryType": { 
					"name": "Query"
				},
				"mutationType": null,
				"subscriptionType": null,
				"types": [{
					"kind": "OBJECT",
					"name": "Query",
					"fields": [{
						"name": "Hello",
						"type": {
							"kind": "SCALAR",
							"name": "String"
						}
					}]
				}]
			}
		}`,
	}
	q2 := &MockSuccessQueryer{
		Value: `{ 
			"__schema": {
				"queryType": { 
					"name": "Query"
				},
				"mutationType": null,
				"subscriptionType": null,
				"types": [{
					"kind": "OBJECT",
					"name": "Query",
					"fields": [{
						"name": "Bye",
						"type": {
							"kind": "SCALAR",
							"name": "String"
						}
					}]
				}]
			}
		}`,
	}
	factory := func(url string) queryer.Queryer {
		if url == "url1" {
			return q1
		}
		return q2
	}
	i := ParallelRemoteSchemaIntrospector{Factory: factory}
	schemas, err := i.IntrospectRemoteSchemas("url1", "url2")
	assert.NoError(t, err)
	assert.Len(t, schemas, 2)
	assert.Equal(t, "Hello", schemas[0].Query.Fields[0].Name)
	assert.Equal(t, "Bye", schemas[1].Query.Fields[0].Name)
}

func TestIntrospectRemoteSchemasRootTypes(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": {
				"name": "Mutation"
			},
			"subscriptionType": {
				"name": "Subscription"
			},
			"types": [{
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "Hello",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					}
				}]
			}, {
				"kind": "OBJECT",
				"name": "Mutation",
				"fields": [{
					"name": "SaveWorld",
					"args": [{
						"description": "My awesome description",
						"name": "name",
						"type": {
							"kind": "NON_NULL",
							"name": null,
							"ofType": {
								"kind": "SCALAR",
								"name": "String",
								"ofType": null
							}
						}
					}],
					"type": {
						"kind": "NON_NULL",
						"name": null,
						"ofType": {
							"kind": "SCALAR",
							"name": "String",
							"ofType": null
						}
					}
				}]
			}, {
				"kind": "OBJECT",
				"name": "Subscription",
				"fields": [{
					"name": "ListenWorld",
					"type": {
						"kind": "SCALAR",
						"name": "String!"
					}
				}]
			}],
			"directives": null
		}
	}`, `
	type Mutation {
		SaveWorld(
			"""
			My awesome description
			"""
			name: String!
		): String!
	}
	type Query {
		Hello: String
	}
	type Subscription {
		ListenWorld: String!
	}
`)
}

func TestIntrospectQueryInterfaces(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"kind": "INTERFACE",
				"name": "MyAwesomeInterface",
				"description": "My description",
				"fields": [{
					"name": "Hello",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					}
				}]
			}, {
				"kind": "OBJECT",
				"name": "MyObject",
				"fields": [{
					"name": "Hello",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					}
				}],
				"interfaces": [{"kind": "INTERFACE", "name": "MyAwesomeInterface"}]
			}, {
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "Hello",
					"type": {
						"kind": "OBJECT",
						"name": "MyObject"
					}
				}]
			}],
			"directives": null
		}
	}`, `
	"""My description"""
	interface MyAwesomeInterface {
		Hello: String
	}
	type MyObject implements MyAwesomeInterface {
		Hello: String
	}
	type Query {
		Hello: MyObject
	}
`)
}

func TestIntrospectQueryUnions(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"kind": "UNION",
				"name": "MyAwesomeUnion",
				"possibleTypes": [{
					"name": "A",
					"kind": "OBJECT"
				}, {
					"name": "B",
					"kind": "OBJECT"
				}]
			}, {
				"kind": "OBJECT",
				"name": "A",
				"fields": [{
					"name": "Hello",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					}
				}]
			}, {
				"kind": "OBJECT",
				"name": "B",
				"fields": [{
					"name": "Bye",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					}
				}]
			}, {
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "Objects",
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
									"kind": "UNION",
									"name": "MyAwesomeUnion"
								}
							}
						}
					}
				}]
			}],
			"directives": null
		}
	}`, `
	union MyAwesomeUnion = A | B 
	type A {
		Hello: String
	}
	type B {
		Bye: String
	}
	type Query {
		Objects: [MyAwesomeUnion!]!
	}`)
}

func TestIntrospectDefaultScalars(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"kind": "SCALAR",
				"name": "String"
			}, {
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "hello",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					}
				}]
			}],
			"directives": null
		}
	}`, `
	type Query {
		hello: String
	}`)
}

func TestIntrospectQueryScalars(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"kind": "SCALAR",
				"name": "Date"
			}, {
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "dates",
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
									"kind": "SCALAR",
									"name": "Date"
								}
							}
						}
					}
				}]
			}],
			"directives": null
		}
	}`, `
	scalar Date 
	type Query {
		dates: [Date!]!
	}`)
}

func TestIntrospectQueryDirectives(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "hello",
					"type": {
						"kind": "SCALAR",
						"name": "ID"
					}
				}]
			}],
			"directives": [{
				"args": [],
				"description": "",
				"locations": [
				    "FIELD_DEFINITION"
				],
				"name": "assertField"
			    }, {
				"args": [],
				"description": "",
				"locations": [
				    "INPUT_FIELD_DEFINITION"
				],
				"name": "assertInputField"
			}, {
				"args": [],
				"description": "",
				"locations": [
				    "INPUT_FIELD_DEFINITION"
				],
				"name": "deprecated"
			}]
		}
	}`, `
	directive @assertField on FIELD_DEFINITION
	directive @assertInputField on INPUT_FIELD_DEFINITION
	type Query {
		hello: ID
	}`)
}

func TestIntrospectQueryEnums(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{ 
		"__schema": {
			"queryType": { 
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"description": "",
				"enumValues": [
				    {
					"deprecationReason": null,
					"description": "",
					"isDeprecated": false,
					"name": "LESS_THAN_YEAR"
				    },
				    {
					"deprecationReason": null,
					"description": "",
					"isDeprecated": false,
					"name": "ONE_TO_TEN"
				    },
				    {
					"deprecationReason": null,
					"description": "",
					"isDeprecated": false,
					"name": "MORE_THAN_TEN"
				    }
				],
				"fields": [],
				"inputFields": [],
				"interfaces": [],
				"kind": "ENUM",
				"name": "WorkExperience",
				"possibleTypes": []
			    }, {
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "workExperience",
					"type": {
						"kind": "ENUM",
						"name": "WorkExperience"
					}
				}]
			}]
		}
	}`, `
	enum WorkExperience {
		LESS_THAN_YEAR
		ONE_TO_TEN
		MORE_THAN_TEN
	}
	type Query {
		workExperience: WorkExperience
	}`)
}

func TestIntrospectQueryInputs(t *testing.T) {
	checkRemoteIntrospectSuccess(t, `{
		"__schema": {
			"queryType": {
				"name": "Query"
			},
			"mutationType": null,
			"subscriptionType": null,
			"types": [{
				"enumValues": [],
				"fields": [],
				"inputFields": [
				    {
					"description": "",
					"name": "stringValue",
					"defaultValue": "string",
					"type": {
					    "kind": "SCALAR",
					    "name": "String"
					}
				    },
				    {
					"description": "",
					"name": "stringListValue",
					"defaultValue": ["1", "2"],
					"type": {
					    "kind": "LIST",
					    "name": null,
					    "ofType": {
						    "kind": "SCALAR",
						    "name": "String"
					    }
					}
				    },
				    {
					"description": "",
					"name": "emptyStringValue",
					"defaultValue": null,
					"type": {
					    "kind": "SCALAR",
					    "name": "String"
					}
				    },
				    {
					"description": "",
					"name": "intValue",
					"defaultValue": 42,
					"type": {
					    "kind": "SCALAR",
					    "name": "Int"
					}
				    },
				    {
					"description": "",
					"name": "emptyIntValue",
					"defaultValue": null,
					"type": {
					    "kind": "SCALAR",
					    "name": "Int"
					}
				    },
				    {
					"description": "",
					"name": "floatValue",
					"defaultValue": 42,
					"type": {
					    "kind": "SCALAR",
					    "name": "Float"
					}
				    },
				    {
					"description": "",
					"name": "floatListValue",
					"defaultValue": [1, 2],
					"type": {
					    "kind": "LIST",
					    "name": null,
					    "ofType": {
						    "kind": "SCALAR",
						    "name": "Float"
					    }
					}
				    },
				    {
					"description": "",
					"name": "booleanValue",
					"defaultValue": true,
					"type": {
					    "kind": "SCALAR",
					    "name": "Boolean"
					}
				    },
				    {
					"description": "",
					"name": "idValue",
					"defaultValue": "someID",
					"type": {
					    "kind": "SCALAR",
					    "name": "ID"
					}
				    }
				],
				"interfaces": [],
				"kind": "INPUT_OBJECT",
				"name": "MyInput",
				"possibleTypes": []
			    }, {
				"kind": "OBJECT",
				"name": "Query",
				"fields": [{
					"name": "hello",
					"type": {
						"kind": "SCALAR",
						"name": "String"
					},
					"args": [{
						"name": "input",
						"type": {
							"kind": "NON_NULL",
							"name": null,
							"ofType": {
								"kind": "INPUT_OBJECT",
								"name": "MyInput",
								"ofType": null
							}
						}
					}]
				}]
			}]
		}
	}`, `
	input MyInput {
		stringValue: String = "string"
		stringListValue: [String] = ["1","2"]
		emptyStringValue: String
		intValue: Int = 42
		emptyIntValue: Int
		floatValue: Float = 42
		floatListValue: [Float] = [1,2]
		booleanValue: Boolean = true
		idValue: ID = "someID"
	}
	type Query {
		hello(input: MyInput!): String
	}`)
}
