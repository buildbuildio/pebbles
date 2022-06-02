package executor

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/buildbuildio/pebbles/common"
	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"

	"github.com/stretchr/testify/assert"
	"github.com/vektah/gqlparser/v2/ast"
)

var parallelExecutor ParallelExecutor

var testOperationName string = "test"

var testRequest = &requests.Request{
	OperationName: &testOperationName,
	Variables:     nil,
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestExecutorSimplePlan(t *testing.T) {
	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"values": []string{
						"hello",
						"world",
					},
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				// this is equivalent to
				// query { values }
				URL:        "0",
				ParentType: "Query",
				SelectionSet: ast.SelectionSet{
					&ast.Field{
						Name: "values",
						Definition: &ast.FieldDefinition{
							Type: ast.ListType(ast.NamedType("String", nil), nil),
						},
					},
				},
			},
			},
		},
	}

	mustCheckEqual(t, ctx, `{ "values": ["hello", "world"]}`)
}

func TestExecutorPlanWithDependencies(t *testing.T) {
	// the query we want to execute is
	// {
	// 		user {                   <- from serviceA
	//      	firstName            <- from serviceA
	// 			favoriteCatPhoto {   <- from serviceB
	// 				url              <- from serviceB
	// 			}
	// 		}
	// }

	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"user": map[string]interface{}{
						"id":        "1",
						"firstName": "hello",
					},
				},
			},
			"1": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"node": map[string]interface{}{
						"favoriteCatPhoto": map[string]interface{}{
							"url": "hello world",
						},
					},
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				{
					// this is equivalent to
					// query { user }
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "user",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("User", nil),
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
					},
					// then we have to ask for the users favorite cat photo and its url
					Then: []*planner.QueryPlanStep{
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"user"},
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "favoriteCatPhoto",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("User", nil),
									},
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "url",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("String", nil),
											},
										},
									},
								},
							}),
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{
		"user": {"id": "1", "firstName": "hello", "favoriteCatPhoto": {"url": "hello world"}}
	}`)
}

func TestExecutorEmptyNode(t *testing.T) {
	// the query we want to execute is
	// {
	//     user {                    <- from serviceA
	//          firstName            <- from serviceA
	//          favoriteCatPhoto {   <- from serviceB, but returns nil
	//              url              <- from serviceB
	// 	        }
	//     }
	// }

	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"user": map[string]interface{}{
						"id":        "1",
						"firstName": "hello",
					},
				},
			},
			"1": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"node": nil,
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				{
					URL: "0",
					// this is equivalent to
					// query { user }
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "user",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("User", nil),
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
					},
					// then we have to ask for the users favorite cat photo and its url
					Then: []*planner.QueryPlanStep{
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"user"},
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "favoriteCatPhoto",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("User", nil),
									},
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "url",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("String", nil),
											},
										},
									},
								},
							}),
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{
		"user": {"id": "1", "firstName": "hello"}
	}`)
}

func TestExecutorInsertIntoInlineFragment(t *testing.T) {
	// the query we want to execute is
	// {
	//   photo {								<- Query.services @ serviceA
	//     ... on Photo {
	// 			createdBy {
	// 				firstName					<- User.firstName @ serviceA
	// 				address						<- User.address @ serviceB
	// 			}
	// 	   }
	//  }
	// }

	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"photo": map[string]interface{}{
						"createdBy": map[string]interface{}{
							"firstName": "John",
							"id":        "1",
						},
					},
				},
			},
			"1": MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for _, input := range inputs {
						assert.Equal(t, map[string]interface{}{"id": "1"}, input.Variables)
						res = append(res, map[string]interface{}{
							"node": map[string]interface{}{
								"address": "addressValue",
							},
						})
					}

					return res, nil
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				// a query to satisfy photo.createdBy.firstName
				{
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Alias: "photo",
							Name:  "photo",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("Photo", nil),
							},
							SelectionSet: ast.SelectionSet{
								&ast.InlineFragment{
									TypeCondition: "Photo",
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "createdBy",
											Definition: &ast.FieldDefinition{
												Name: "createdBy",
												Type: ast.NamedType("User", nil),
											},
											SelectionSet: ast.SelectionSet{
												&ast.Field{
													Name: "firstName",
													Definition: &ast.FieldDefinition{
														Name: "firstName",
														Type: ast.NamedType("String", nil),
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Then: []*planner.QueryPlanStep{
						// a query to satisfy User.address
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"photo", "createdBy"}, // photo is the query name here
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "address",
									Definition: &ast.FieldDefinition{
										Name: "address",
										Type: ast.NamedType("String", nil),
									},
								},
							}),
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{
		"photo": {
			"createdBy": {
				"id":        "1",
				"firstName": "John",
				"address":   "addressValue"
			}
		}
	}`)
}

func TestExecutorInsertIntoListInlineFragments(t *testing.T) {
	// {
	// 	photos {								<-- Query.services @ serviceA, list
	// 	  ... on Photo {
	// 		createdBy {
	// 	  	  firstName								<-- User.firstName @ serviceA
	// 	  	  address								<-- User.address @ serviceB
	// 	    }
	// 	   }
	//  }
	// }
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"photos": []interface{}{
						map[string]interface{}{
							"createdBy": map[string]interface{}{
								"firstName": "John",
								"id":        "1",
							},
						},
						map[string]interface{}{
							"createdBy": map[string]interface{}{
								"firstName": "Jane",
								"id":        "2",
							},
						},
					},
				},
			},
			"1": MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for _, input := range inputs {
						assert.Contains(t, []interface{}{"1", "2"}, input.Variables["id"])
						res = append(res, map[string]interface{}{
							"node": map[string]interface{}{
								"address": fmt.Sprintf("address-%s", input.Variables["id"]),
							},
						})
					}

					return res, nil
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				// a query to satisfy photo.createdBy.firstName
				{
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Alias: "photos",
							Name:  "photos",
							Definition: &ast.FieldDefinition{
								Type: ast.ListType(ast.NamedType("Photo", nil), nil),
							},
							SelectionSet: ast.SelectionSet{
								&ast.InlineFragment{
									TypeCondition: "Photo",
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "createdBy",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("User", nil),
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
									},
								},
							},
						},
					},
					Then: []*planner.QueryPlanStep{
						// a query to satisfy User.address
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"photos", "createdBy"}, // photo is the query name here
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "address",
									Definition: &ast.FieldDefinition{
										Name: "address",
										Type: ast.NamedType("String", nil),
									},
								},
							}),
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{
		"photos": [
			{
				"createdBy": {
					"id":        "1",
					"firstName": "John",
					"address":   "address-1"
				}
			},
			{
				"createdBy": {
					"id":        "2",
					"firstName": "Jane",
					"address":   "address-2"
				}
			}
		]
	}`)
}

func TestExecutorInsertIntoInlineFragmentsList(t *testing.T) {
	// {
	// 	photo {								<-- Query.services @ serviceA
	// 	  ... on Photo {
	//       viewedBy {						<-- list
	//          firstName					<-- User.firstName @ serviceA
	//          address						<-- User.address @ serviceB
	//       }
	//    }
	// 	}
	// }
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"photo": map[string]interface{}{
						"viewedBy": []interface{}{
							map[string]interface{}{
								"firstName": "John",
								"id":        "1",
							},
							map[string]interface{}{
								"firstName": "Jane",
								"id":        "2",
							},
						},
					},
				},
			},
			"1": MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for _, input := range inputs {
						assert.Contains(t, []interface{}{"1", "2"}, input.Variables["id"])
						res = append(res, map[string]interface{}{
							"node": map[string]interface{}{
								"address": fmt.Sprintf("address-%s", input.Variables["id"]),
							},
						})
					}

					return res, nil
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				// a query to satisfy photo.viewedBy.firstName
				{
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Alias: "photo",
							Name:  "photo",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("Photo", nil),
							},
							SelectionSet: ast.SelectionSet{
								&ast.InlineFragment{
									TypeCondition: "Photo",
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "viewedBy",
											Definition: &ast.FieldDefinition{
												Type: ast.ListType(ast.NamedType("User", nil), nil),
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
									},
								},
							},
						},
					},
					Then: []*planner.QueryPlanStep{
						// a query to satisfy User.address
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"photo", "viewedBy"},
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "address",
									Definition: &ast.FieldDefinition{
										Name: "address",
										Type: ast.NamedType("String", nil),
									},
								},
							}),
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{
		"photo": {
			"viewedBy": [
				{
					"firstName": "John",
					"id":        "1",
					"address":   "address-1"
				},
				{
					"firstName": "Jane",
					"id":        "2",
					"address":   "address-2"
				}
			]
		}
	}`)
}

func TestExecutorInsertIntoLists(t *testing.T) {
	// the query we want to execute is
	// {
	// 	users {                  	<- Query.services @ serviceA
	//      	firstName
	//          	friends {				<- list
	//              	firstName
	//              	photoGallery {   	<- list, User.photoGallery @ serviceB
	// 				url
	// 				followers { .   <- list
	//                  			firstName	<- User.firstName @ serviceA
	//                 		}
	// 			}
	//           	}
	// 	}
	// }

	// values to test against
	photoGalleryURL := "photoGalleryURL"
	followerName := "John"

	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					if len(inputs) > 0 && !strings.Contains(inputs[0].Query, "node") {
						return []map[string]interface{}{{
							"users": []interface{}{
								map[string]interface{}{
									"firstName": "hello",
									"friends": []interface{}{
										map[string]interface{}{
											"firstName": "John",
											"id":        "1",
										},
										map[string]interface{}{
											"firstName": "Jacob",
											"id":        "2",
										},
									},
								},
								map[string]interface{}{
									"firstName": "goodbye",
									"friends": []interface{}{
										map[string]interface{}{
											"firstName": "Jingleheymer",
											"id":        "3",
										},
										map[string]interface{}{
											"firstName": "Schmidt",
											"id":        "4",
										},
									},
								},
							},
						}}, nil
					}

					var res []map[string]interface{}
					for _, input := range inputs {
						// make sure that we got the right variable inputs
						assert.Equal(t, map[string]interface{}{"id": "1"}, input.Variables)

						res = append(res, map[string]interface{}{
							"node": map[string]interface{}{
								"firstName": followerName,
							},
						})
					}

					// return the payload
					return res, nil
				},
			},
			"1": MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for range inputs {
						res = append(res, map[string]interface{}{
							"node": map[string]interface{}{
								"photoGallery": []interface{}{
									map[string]interface{}{
										"url": photoGalleryURL,
										"followers": []interface{}{
											map[string]interface{}{
												"id": "1",
											},
										},
									},
								},
							},
						})
					}

					// return the payload
					return res, nil
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				{
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "users",
							Definition: &ast.FieldDefinition{
								Type: ast.ListType(ast.NamedType("User", nil), nil),
							},
							SelectionSet: ast.SelectionSet{
								&ast.Field{
									Name: "firstName",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("String", nil),
									},
								},
								&ast.Field{
									Name: "friends",
									Definition: &ast.FieldDefinition{
										Type: ast.ListType(ast.NamedType("User", nil), nil),
									},
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("String", nil),
											},
											Name: "firstName",
										},
									},
								},
							},
						},
					},
					// then we have to ask for the users photo gallery
					Then: []*planner.QueryPlanStep{
						// a query to satisfy User.photoGallery
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"users", "friends"},
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "photoGallery",
									Definition: &ast.FieldDefinition{
										Type: ast.ListType(ast.NamedType("CatPhoto", nil), nil),
									},
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "url",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("String", nil),
											},
										},
										&ast.Field{
											Name: "followers",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("User", nil),
											},
											SelectionSet: ast.SelectionSet{},
										},
									},
								},
							}),
							Then: []*planner.QueryPlanStep{
								// a query to satisfy User.firstName
								{
									URL:            "0",
									ParentType:     "User",
									InsertionPoint: []string{"users", "friends", "photoGallery", "followers"},
									SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
										&ast.Field{
											Name: "firstName",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("String", nil),
											},
										},
									}),
								},
							},
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{
		"users": [
			{
				"firstName": "hello",
				"friends": [
					{
						"firstName": "John",
						"id":        "1",
						"photoGallery": [
							{
								"url": "photoGalleryURL",
								"followers": [
									{
										"id":        "1",
										"firstName": "John"
									}
								]
							}
						]
					},
					{
						"firstName": "Jacob",
						"id":        "2",
						"photoGallery": [
							{
								"url": "photoGalleryURL",
								"followers": [
									{
										"id":        "1",
										"firstName": "John"
									}
								]
							}
						]
					}
				]
			},
			{
				"firstName": "goodbye",
				"friends": [
					{
						"firstName": "Jingleheymer",
						"id":        "3",
						"photoGallery": [
							{
								"url": "photoGalleryURL",
								"followers": [
									{
										"id":        "1",
										"firstName": "John"
									}
								]
							}
						]
					},
					{
						"firstName": "Schmidt",
						"id":        "4",
						"photoGallery": [
							{
								"url": "photoGalleryURL",
								"followers": [
									{
										"id":        "1",
										"firstName": "John"
									}
								]
							}
						]
					}
				]
			}
		]
	}`)
}

func TestExecutorMultipleErrors(t *testing.T) {
	// an executor should return a list of every error that it encounters while executing the plan

	// build a query plan that the executor will follow
	_, err := parallelExecutor.Execute(&ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					return nil, errors.New("err1")
				},
			},
			"1": &MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					errs := gqlerrors.ExtendErrorList(nil, errors.New("err2"))
					errs = gqlerrors.ExtendErrorList(errs, errors.New("err3"))
					return nil, errs
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				{
					// this is equivalent to
					// query { values }
					URL:        "0",
					ParentType: "Query",
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "values",
							Definition: &ast.FieldDefinition{
								Type: ast.ListType(ast.NamedType("String", nil), nil),
							},
						},
					},
				},
				{
					// this is equivalent to
					// query { values }
					URL:        "1",
					ParentType: "Query",
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "values",
							Definition: &ast.FieldDefinition{
								Type: ast.ListType(ast.NamedType("String", nil), nil),
							},
						},
					},
				},
			},
		},
	})
	assert.Error(t, err, "Did not encounter error executing plan")

	// since 3 errors were thrown we need to make sure we actually received an error list
	list, ok := err.(gqlerrors.ErrorList)
	assert.True(t, ok, "Error was not an error list")

	assert.Len(t, list, 3, "Error list did not have 3 items")
}

func TestExecutorIncludeIf(t *testing.T) {
	// the query we want to execute is
	// {
	// 		user @include(if: false) {   <- from serviceA
	// 			favoriteCatPhoto {   	 <- from serviceB
	// 				url              	 <- from serviceB
	// 			}
	// 		}
	// }

	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{},
			},
			"1": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"node": map[string]interface{}{
						"favoriteCatPhoto": map[string]interface{}{
							"url": "hello world",
						},
					},
				},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				{
					// this is equivalent to
					// query { user }
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "user",
							Definition: &ast.FieldDefinition{
								Type: ast.NamedType("User", nil),
							},
							SelectionSet: ast.SelectionSet{
								&ast.Field{
									Name: "firstName",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("String", nil),
									},
								},
							},
							Directives: ast.DirectiveList{
								&ast.Directive{
									Name: "include",
									Arguments: ast.ArgumentList{
										&ast.Argument{
											Name: "if",
											Value: &ast.Value{
												Kind: ast.BooleanValue,
												Raw:  "true",
											},
										},
									},
								},
							},
						},
					},
					// then we have to ask for the users favorite cat photo and its url
					Then: []*planner.QueryPlanStep{
						{
							URL:            "1",
							ParentType:     "User",
							InsertionPoint: []string{"user"},
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name: "favoriteCatPhoto",
									Definition: &ast.FieldDefinition{
										Type: ast.NamedType("User", nil),
									},
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name: "url",
											Definition: &ast.FieldDefinition{
												Type: ast.NamedType("String", nil),
											},
										},
									},
								},
							}),
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{}`)
}

func TestExecutorQueryVariables(t *testing.T) {
	// the variables we'll be threading through
	fullVariables := map[string]interface{}{
		"hello":   "world",
		"goodbye": "moon",
	}

	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for _, input := range inputs {
						// make sure that we got the right variable inputs
						assert.Equal(t, map[string]interface{}{"hello": "world"}, input.Variables)
						// and definitions
						assert.Equal(t, fmt.Sprintf("query %s ($hello: [String]) {\n\tvalues(filter: $hello)\n}", testOperationName), input.Query)
						assert.Equal(t, testOperationName, *input.OperationName)
						res = append(res, map[string]interface{}{"values": []string{"world"}})
					}

					return res, nil
				},
			},
		},
		Request: &requests.Request{
			OperationName: &testOperationName,
			Variables:     fullVariables,
		},
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{
				{
					// this is equivalent to
					// query { values (filter: $hello) }
					URL:           "0",
					ParentType:    "Query",
					OperationName: &testOperationName,
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name: "values",
							Definition: &ast.FieldDefinition{
								Arguments: ast.ArgumentDefinitionList{
									&ast.ArgumentDefinition{
										Name: "filter",
										Type: ast.ListType(ast.NamedType("String", nil), nil),
									},
								},
								Type: ast.ListType(ast.NamedType("String", nil), nil),
							},
							Arguments: ast.ArgumentList{
								{
									Name: "filter",
									Value: &ast.Value{
										Raw: "hello",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	mustCheckEqual(t, ctx, `{"values": ["world"]}`)
}

// TestExecutor_plansWithManyDeepDependencies test that two `Then` works without races
// Test provides quite deep query to illustrate problem with
// reused memory during slice `appends`
func TestExecutorPlansWithManyDeepDependencies(t *testing.T) {
	// the query we want to execute is
	//	{
	//		user {						<- serviceA
	//			parent {				<- serviceA
	//				parent {			<- serviceA
	//					id  			<- serviceA
	//					house {			<- serviceB
	//						address		<- serviceC
	//						cats {		<- serviceC
	//							id  	<- serviceC
	//							name	<- serviceD
	//						}
	//					}
	//				}
	//			}
	//		}
	//	}

	// test helpers
	definitionFactory := func(s string) *ast.FieldDefinition {
		return &ast.FieldDefinition{
			Type: ast.NamedType(s, nil),
		}
	}
	definitionListFactory := func(s string) *ast.FieldDefinition {
		return &ast.FieldDefinition{
			Type: ast.ListType(ast.NamedType(s, nil), nil),
		}
	}
	idFieldFactory := func() *ast.Field {
		return &ast.Field{
			Name:       common.IDFieldName,
			Definition: definitionFactory("ID"),
		}
	}

	// build a query plan that the executor will follow
	ctx := &ExecutionContext{
		Queryers: map[string]queryer.Queryer{
			"0": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"user": map[string]interface{}{
						"parent": map[string]interface{}{
							"parent": map[string]interface{}{
								"id": "1",
							},
						},
					},
				},
			},
			"1": &MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for range inputs {
						res = append(res, map[string]interface{}{"node": map[string]interface{}{
							"house": map[string]interface{}{
								"id": "2",
								"cats": []interface{}{
									map[string]interface{}{"id": "3"},
								},
							},
						}})
					}

					return res, nil
				},
			},
			"2": &MockQueryerFunc{
				F: func(inputs []*requests.Request) ([]map[string]interface{}, error) {
					var res []map[string]interface{}
					for range inputs {
						res = append(res, map[string]interface{}{
							"node": map[string]interface{}{
								"id":      "2",
								"address": "Cats street",
							},
						})
					}

					return res, nil
				},
			},
			"3": &MockSuccessQueryer{
				Value: map[string]interface{}{
					"node": map[string]interface{}{
						"id": "3", "name": "kitty",
					}},
			},
		},
		Request: testRequest,
		QueryPlan: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{

				{
					URL:            "0",
					ParentType:     "Query",
					InsertionPoint: []string{},
					SelectionSet: ast.SelectionSet{
						&ast.Field{
							Name:       "user",
							Definition: definitionFactory("User"),
							SelectionSet: ast.SelectionSet{
								&ast.Field{
									Name:       "parent",
									Definition: definitionFactory("User"),
									SelectionSet: ast.SelectionSet{
										&ast.Field{
											Name:       "parent",
											Definition: definitionFactory("User"),
											SelectionSet: ast.SelectionSet{
												idFieldFactory(),
											},
										},
									},
								},
							},
						},
					},
					Then: []*planner.QueryPlanStep{
						{
							URL:            "1",
							ParentType:     "House",
							InsertionPoint: []string{"user", "parent", "parent"},
							SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
								&ast.Field{
									Name:       "house",
									Definition: definitionFactory("House"),
									SelectionSet: ast.SelectionSet{
										idFieldFactory(),
										&ast.Field{
											Name:       "cats",
											Definition: definitionListFactory("Cat"),
											SelectionSet: ast.SelectionSet{
												idFieldFactory(),
											},
										},
									},
								},
							}),

							Then: []*planner.QueryPlanStep{
								{
									URL:            "2",
									ParentType:     "House",
									InsertionPoint: []string{"user", "parent", "parent", "house"},
									SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
										idFieldFactory(),
										&ast.Field{
											Name:       "address",
											Definition: definitionFactory("String"),
										},
									}),
								},
								{
									URL:            "3",
									ParentType:     "User",
									InsertionPoint: []string{"user", "parent", "parent", "house", "cats"},
									SelectionSet: selectionSetWithNodeDef(ast.SelectionSet{
										idFieldFactory(),
										&ast.Field{
											Name:       "name",
											Definition: definitionFactory("String"),
										},
									}),
								},
							},
						},
					},
				},
			},
		},
	}
	mustCheckEqual(t, ctx, `{
		"user": {
			"parent": {
				"parent": {
					"id": "1",
					"house": {
						"id":      "2",
						"address": "Cats street",
						"cats": [
							{"id": "3", "name": "kitty"}
						]
					}
				}
			}
		}
	}`)
}
