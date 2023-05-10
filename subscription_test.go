package pebbles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/queryer"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type MockQueryer struct {
	ResCh chan *requests.Response
}

func (MockQueryer) Query(_ []*requests.Request) ([]map[string]interface{}, error) {
	return nil, nil
}

func (MockQueryer) URL() string {
	return ""
}

func (m MockQueryer) Subscribe(_ *requests.Request, _ <-chan struct{}, resCh chan *requests.Response) error {
	go func() {
		res := <-m.ResCh
		resCh <- res
	}()
	return nil
}

func TestGatewaySubscriptionSimple(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				URL:        "0",
				ParentType: "Subscription",
				SelectionSet: ast.SelectionSet{&ast.Field{
					Name:         "test",
					SelectionSet: nil,
				}},
				InsertionPoint: nil,
				Then:           nil,
			}},
		},
	}
	expectedResp := map[string]interface{}{
		"test": "YES",
	}
	me := &MockExecutor{
		Res: expectedResp,
	}
	schema := `
		type Subscription {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}

	mq := &MockQueryer{
		ResCh: make(chan *requests.Response),
	}

	gw, err := NewGateway(
		[]string{""},
		WithExecutor(me),
		WithRemoteSchemaIntrospector(mi),
		WithPlanner(mp),
		WithQueryerFactory(func(pc *planner.PlanningContext, s string) queryer.Queryer {
			return mq
		}),
	)
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(gw.Handler))

	dialer := ws.Dialer{
		Timeout:   time.Second,
		Protocols: []string{"graphql-ws"},
	}

	url := strings.Replace(server.URL, "http", "ws", 1)

	conn, _, _, err := dialer.Dial(context.Background(), url)
	require.NoError(t, err)

	bInitMsg, _ := json.Marshal(requests.ClientSubMsg{
		Type: requests.SubConnectionInit,
	})

	err = wsutil.WriteClientText(conn, bInitMsg)
	require.NoError(t, err)
	payload := &requests.Request{
		Query: "subscription {test}",
	}
	bRequestMsg, _ := json.Marshal(requests.ClientSubMsg{
		Type:    requests.SubStart,
		ID:      "1",
		Payload: payload,
	})

	err = wsutil.WriteClientText(conn, bRequestMsg)
	require.NoError(t, err)

	doneCh := make(chan bool)
	go func() {
		for {
			msg, _ := wsutil.ReadServerText(conn)
			go func() {
				mq.ResCh <- &requests.Response{
					Data: expectedResp,
				}
			}()

			var serverResp requests.ServerSubMsg
			json.Unmarshal(msg, &serverResp)

			switch serverResp.Type {
			case requests.SubComplete,
				requests.SubConnectionError,
				requests.SubConnectionTerminate,
				requests.SubError:
				require.FailNow(t, "wrong event")
				return
			case requests.SubData:
				require.EqualValues(t, expectedResp, serverResp.Payload.Data)
				doneCh <- true
				return
			}
		}
	}()

	select {
	case <-doneCh:
		break
	case <-time.Tick(time.Second * 5):
		require.FailNow(t, "timeout")
	}

	// stop connection
	msg, _ := json.Marshal(requests.ClientSubMsg{
		Type: requests.SubStop,
		ID:   "1",
	})

	err = wsutil.WriteClientText(conn, msg)
	require.NoError(t, err)

	// terminate connection
	msg, _ = json.Marshal(requests.ClientSubMsg{
		Type: requests.SubConnectionTerminate,
	})

	err = wsutil.WriteClientText(conn, msg)
	require.NoError(t, err)

	// connection should be closed
	_, _, err = wsutil.ReadServerData(conn)
	require.Error(t, err)
}

func TestGatewaySubscriptionWithSubqueries(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				URL:        "0",
				ParentType: "Subscription",
				SelectionSet: ast.SelectionSet{&ast.Field{
					Name: "test",
					Definition: &ast.FieldDefinition{
						Name: "Entry",
						Type: ast.NamedType("Entry", nil),
					},
					SelectionSet: ast.SelectionSet{&ast.Field{
						Name:         "id",
						SelectionSet: nil,
					}},
				}},
				InsertionPoint: nil,
				Then: []*planner.QueryPlanStep{{
					URL:        "1",
					ParentType: "Entry",
					SelectionSet: ast.SelectionSet{&ast.Field{
						Name:         "field",
						SelectionSet: nil,
					}},
					InsertionPoint: []string{"test"},
				}},
			}},
		},
	}
	expectedSubResp := map[string]interface{}{
		"test": map[string]interface{}{
			"id": "1",
		},
	}

	expectedFullResp := map[string]interface{}{
		"test": map[string]interface{}{
			"id":    "1",
			"field": "12345",
		},
	}
	me := &MockExecutor{
		Res: expectedFullResp,
	}
	schema := `
		type Subscription {
			test: Entry!
		}

		type Entry {
			id: ID!
			field: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}

	mq := &MockQueryer{
		ResCh: make(chan *requests.Response),
	}

	gw, err := NewGateway(
		[]string{""},
		WithExecutor(me),
		WithRemoteSchemaIntrospector(mi),
		WithPlanner(mp),
		WithQueryerFactory(func(pc *planner.PlanningContext, s string) queryer.Queryer {
			return mq
		}),
	)
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(gw.Handler))

	dialer := ws.Dialer{
		Timeout:   time.Second,
		Protocols: []string{"graphql-ws"},
	}

	url := strings.Replace(server.URL, "http", "ws", 1)

	conn, _, _, err := dialer.Dial(context.Background(), url)
	require.NoError(t, err)

	bInitMsg, _ := json.Marshal(requests.ClientSubMsg{
		Type: requests.SubConnectionInit,
	})

	err = wsutil.WriteClientText(conn, bInitMsg)
	require.NoError(t, err)
	payload := &requests.Request{
		Query: "subscription {test {id field}}",
	}
	bRequestMsg, _ := json.Marshal(requests.ClientSubMsg{
		Type:    requests.SubStart,
		ID:      "1",
		Payload: payload,
	})

	err = wsutil.WriteClientText(conn, bRequestMsg)
	require.NoError(t, err)

	doneCh := make(chan bool)
	go func() {
		for {
			msg, _ := wsutil.ReadServerText(conn)
			go func() {
				mq.ResCh <- &requests.Response{
					Data: expectedSubResp,
				}
			}()

			var serverResp requests.ServerSubMsg
			json.Unmarshal(msg, &serverResp)

			switch serverResp.Type {
			case requests.SubComplete,
				requests.SubConnectionError,
				requests.SubConnectionTerminate,
				requests.SubError:
				require.FailNow(t, "wrong event")
				return
			case requests.SubData:
				require.EqualValues(t, expectedFullResp, serverResp.Payload.Data)
				doneCh <- true
				return
			}
		}
	}()

	select {
	case <-doneCh:
		break
	case <-time.Tick(time.Second * 5):
		require.FailNow(t, "timeout")
	}

	// terminate connection
	msg, _ := json.Marshal(requests.ClientSubMsg{
		Type: requests.SubConnectionTerminate,
	})

	err = wsutil.WriteClientText(conn, msg)
	require.NoError(t, err)

	// connection should be closed
	_, _, err = wsutil.ReadServerData(conn)
	require.Error(t, err)
}

func TestGatewaySubscriptionUnsupportedMessage(t *testing.T) {
	mp := &MockPlanner{
		Res: &planner.QueryPlan{
			RootSteps: []*planner.QueryPlanStep{{
				URL:        "0",
				ParentType: "Subscription",
				SelectionSet: ast.SelectionSet{&ast.Field{
					Name:         "test",
					SelectionSet: nil,
				}},
				InsertionPoint: nil,
				Then:           nil,
			}},
		},
	}
	expectedResp := map[string]interface{}{
		"test": "YES",
	}
	me := &MockExecutor{
		Res: expectedResp,
	}
	schema := `
		type Subscription {
			test: String!
		}
	`

	s := gqlparser.MustLoadSchema(&ast.Source{Name: "fixture", Input: schema})
	mi := &MockRemoteSchemaIntrospector{Res: []*ast.Schema{s}}

	mq := &MockQueryer{
		ResCh: make(chan *requests.Response),
	}

	gw, err := NewGateway(
		[]string{""},
		WithExecutor(me),
		WithRemoteSchemaIntrospector(mi),
		WithPlanner(mp),
		WithQueryerFactory(func(pc *planner.PlanningContext, s string) queryer.Queryer {
			return mq
		}),
	)
	assert.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(gw.Handler))

	dialer := ws.Dialer{
		Timeout:   time.Second,
		Protocols: []string{"wrong"},
	}

	url := strings.Replace(server.URL, "http", "ws", 1)

	conn, _, _, err := dialer.Dial(context.Background(), url)
	require.NoError(t, err)

	bInitMsg, _ := json.Marshal(requests.ClientSubMsg{
		Type: "bla",
	})

	err = wsutil.WriteClientText(conn, bInitMsg)
	require.NoError(t, err)

	_, _, err = wsutil.ReadServerData(conn)
	require.Error(t, err)
}
