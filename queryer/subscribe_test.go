package queryer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buildbuildio/pebbles/gqlerrors"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribe(t *testing.T) {
	var called int32
	testFn := func() {
		atomic.AddInt32(&called, 1)
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test", r.Header.Get("test"))
		upgrader := ws.HTTPUpgrader{
			Timeout: time.Second * 60,
			Protocol: func(subprotocol string) bool {
				return string(subprotocol) == "graphql-ws"
			},
		}

		conn, _, _, err := upgrader.Upgrade(r, w)
		if err != nil {
			return
		}

		for {
			msg, err := wsutil.ReadClientText(conn)
			if err != nil {
				return
			}

			var subMsg requests.ClientSubMsg
			if err := json.Unmarshal(msg, &subMsg); err != nil {
				return
			}

			switch subMsg.Type {
			// When the GraphQL WS connection is initiated, send an ACK back
			case requests.SubConnectionInit:
				resp := requests.ServerSubMsg{
					Type: requests.SubConnectionAck,
				}
				bresp, err := json.Marshal(resp)
				if err != nil {
					return
				}
				if err := wsutil.WriteServerText(conn, bresp); err != nil {
					return
				}

				testFn()

			// Let event handlers deal with starting operations
			case requests.SubStart:
				resp := requests.ServerSubMsg{
					Type: requests.SubData,
					Payload: &requests.Response{
						Data: map[string]interface{}{"hello": "world"},
					},
				}
				bresp, err := json.Marshal(resp)
				if err != nil {
					return
				}
				if err := wsutil.WriteServerText(conn, bresp); err != nil {
					return
				}
				testFn()
				resp = requests.ServerSubMsg{
					Type: requests.SubComplete,
				}
				bresp, err = json.Marshal(resp)
				if err != nil {
					return
				}
				if err := wsutil.WriteServerText(conn, bresp); err != nil {
					return
				}
			}
		}

	}))
	defer s.Close()

	queryer := NewMultiOpQueryer(s.URL, 3)

	ctx := context.WithValue(context.Background(), "key", "value")

	queryer.WithContext(ctx)

	queryer.WithMiddlewares([]RequestMiddleware{func(r *http.Request) error {
		// check context
		c := r.Context()
		require.Equal(t, "value", c.Value("key"))
		// set header to test later
		r.Header.Set("test", "test")
		return nil
	}})

	closeCh := make(chan struct{})
	resCh := make(chan *requests.Response)

	err := queryer.Subscribe(&requests.Request{
		Query: "test",
	}, closeCh, resCh)
	require.NoError(t, err)
	// query
	select {
	case res := <-resCh:
		assert.EqualValues(t, requests.Response{
			Data: map[string]interface{}{"hello": "world"},
		}, *res)
	case <-time.After(time.Millisecond * 50):
		assert.FailNow(t, "timeout")
	}
	closeCh <- struct{}{}
	assert.EqualValues(t, 2, atomic.LoadInt32(&called))
}

func TestSubscribeErrorQuery(t *testing.T) {
	var called int32
	testFn := func() {
		atomic.AddInt32(&called, 1)
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := ws.HTTPUpgrader{
			Timeout: time.Second * 60,
			Protocol: func(subprotocol string) bool {
				return string(subprotocol) == "graphql-ws"
			},
		}

		conn, _, _, err := upgrader.Upgrade(r, w)
		if err != nil {
			return
		}

		for {
			msg, err := wsutil.ReadClientText(conn)
			if err != nil {
				return
			}

			var subMsg requests.ClientSubMsg
			if err := json.Unmarshal(msg, &subMsg); err != nil {
				return
			}

			switch subMsg.Type {
			// When the GraphQL WS connection is initiated, send an ACK back
			case requests.SubConnectionInit:
				resp := requests.ServerSubMsg{
					Type: requests.SubConnectionAck,
				}
				bresp, err := json.Marshal(resp)
				if err != nil {
					return
				}
				if err := wsutil.WriteServerText(conn, bresp); err != nil {
					return
				}

				testFn()

			// Let event handlers deal with starting operations
			case requests.SubStart:
				resp := requests.ServerSubErorrMsg{
					Type:    requests.SubError,
					Payload: gqlerrors.FormatError(errors.New("test err")),
				}

				bresp, err := json.Marshal(resp)
				if err != nil {
					return
				}
				if err := wsutil.WriteServerText(conn, bresp); err != nil {
					return
				}
				testFn()
			}
		}

	}))
	defer s.Close()

	queryer := NewMultiOpQueryer(s.URL, 3)

	closeCh := make(chan struct{})
	resCh := make(chan *requests.Response)

	err := queryer.Subscribe(&requests.Request{
		Query: "test",
	}, closeCh, resCh)
	require.NoError(t, err)
	// query
	select {
	case res := <-resCh:
		assert.EqualValues(t, requests.Response{
			Errors: gqlerrors.FormatError(errors.New("test err")),
		}, *res)
	case <-time.After(time.Millisecond * 50):
		assert.FailNow(t, "timeout")
	}
	closeCh <- struct{}{}
	assert.EqualValues(t, 2, atomic.LoadInt32(&called))
}

func TestSubscribeErrorNoSupportForWs(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		return
	}))
	defer s.Close()

	queryer := NewMultiOpQueryer(s.URL, 3)

	closeCh := make(chan struct{})
	resCh := make(chan *requests.Response)

	err := queryer.Subscribe(&requests.Request{
		Query: "test",
	}, closeCh, resCh)
	require.Error(t, err)
}
