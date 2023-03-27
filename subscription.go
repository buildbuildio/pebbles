package pebbles

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/buildbuildio/pebbles/planner"
	"github.com/buildbuildio/pebbles/requests"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type subscriptionDict map[string]*subscriptionEntry

func (sd subscriptionDict) Clean(key string) {
	if subEntry, ok := sd[key]; ok {
		go subEntry.Close()
		delete(sd, key)
	}
}

func (sd subscriptionDict) CleanAll() {
	for key := range sd {
		sd.Clean(key)
	}
}

func (sd subscriptionDict) isAllClosed() bool {
	isAllClosed := len(sd) != 0
	for _, v := range sd {
		if v.isClosed {
			isAllClosed = isAllClosed && v.isClosed
		}
	}
	return isAllClosed
}

func sendHeartbeat(conn net.Conn, closeCh <-chan struct{}) error {
	timeTicker := time.NewTicker(time.Second * 4)
	defer timeTicker.Stop()

	bMsg, err := json.Marshal(requests.ServerSubMsg{Type: requests.SubConnectionKeepAlive})
	if err != nil {
		return err
	}
	for {
		select {
		case <-timeTicker.C:
			if err := wsutil.WriteServerText(conn, bMsg); err != nil {
				return err
			}
		case <-closeCh:
			return nil
		}
	}
}

func (g *Gateway) subscriptionHandler(w http.ResponseWriter, r *http.Request) {
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

	subDict := make(subscriptionDict)

	closeCh := make(chan struct{})

	stopConnFn := func() {
		defer func() {
			recover()
		}()
		// gracefully close connection
		body := ws.NewCloseFrameBody(ws.StatusNormalClosure, "")
		frame := ws.NewCloseFrame(body)
		if err := ws.WriteHeader(conn, frame.Header); err != nil {
			return
		}
		if _, err := conn.Write(body); err != nil {
			return
		}

		// send close to goroutine
		closeCh <- struct{}{}
		close(closeCh)

		// close all running handlers
		subDict.CleanAll()

		// close conn
		conn.Close()
	}

	defer stopConnFn()

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
			// start sending heartbeat
			go sendHeartbeat(conn, closeCh)

		// Let event handlers deal with starting operations
		case requests.SubStart:
			request := subMsg.Payload
			request.Original = r

			query, qerr := gqlparser.LoadQuery(g.schema, request.Query)
			if qerr != nil {
				return
			}

			var operation *ast.OperationDefinition
			if request.OperationName != nil {
				operation = query.Operations.ForName(*request.OperationName)
			} else if len(query.Operations) == 1 {
				operation = query.Operations[0]
			}

			if operation == nil {
				return
			}

			planningContext := &planner.PlanningContext{
				Request:    request,
				Operation:  operation,
				Schema:     g.schema,
				TypeURLMap: g.typeURLMap,
			}

			subEntry, err := g.newSubscriptionEntry(subMsg.ID, planningContext)
			if err != nil {
				return
			}

			subDict[subMsg.ID] = subEntry

			go subEntry.Listen(conn)

		// Stop running operations
		case requests.SubStop:
			subDict.Clean(subMsg.ID)

		// When the GraphQL WS connection is terminated by the client,
		// close the connection and close all the running operations
		case requests.SubConnectionTerminate:
			subDict.CleanAll()
			return

		// GraphQL WS protocol messages that are not handled represent
		// a bug in our implementation; make this very obvious by logging
		// an error
		default:
			fmt.Println("Unknown message", string(msg))
		}
	}
}
