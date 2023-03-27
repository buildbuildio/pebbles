package queryer

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/buildbuildio/pebbles/requests"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func (q *MultiOpQueryer) Subscribe(req *requests.Request, closeCh <-chan struct{}, resCh chan *requests.Response) error {
	r := &http.Request{
		Header: make(http.Header),
	}
	if q.ctx != nil {
		r = r.WithContext(q.ctx)
	}
	for _, mw := range q.mdwares {
		if err := mw(r); err != nil {
			return err
		}
	}

	dialer := ws.Dialer{
		Timeout:   time.Second,
		Protocols: []string{"graphql-ws"},
		Header:    ws.HandshakeHeaderHTTP(r.Header),
	}

	parsedURL, err := url.Parse(q.url)
	if err != nil {
		return err
	}

	parsedURL.Scheme = "ws"

	conn, _, _, err := dialer.Dial(q.ctx, parsedURL.String())
	if err != nil {
		return err
	}

	errCh := make(chan error)
	defer close(errCh)

	go func() {
		defer func() {
			recover()
		}()
		<-closeCh
		conn.Close()
	}()

	go func() {
		defer func() {
			defer func() {
				recover()
			}()
			conn.Close()
			// indicate that it's done
			resCh <- nil
		}()

		bInitMsg, err := json.Marshal(requests.ClientSubMsg{
			Type: requests.SubConnectionInit,
		})
		if err != nil {
			errCh <- err
			return
		}

		// send init msg
		if err := wsutil.WriteClientText(conn, bInitMsg); err != nil {
			errCh <- err
			return
		}

		bRequestMsg, err := json.Marshal(requests.ClientSubMsg{
			Type:    requests.SubStart,
			ID:      "1",
			Payload: req,
		})
		if err != nil {
			errCh <- err
			return
		}
		// send query msg
		if err := wsutil.WriteClientText(conn, bRequestMsg); err != nil {
			errCh <- err
			return
		}

		// init proccess is done
		errCh <- nil

		for {
			msg, err := wsutil.ReadServerText(conn)
			if err != nil {
				return
			}

			var serverResp requests.ServerSubMsg
			if err := json.Unmarshal(msg, &serverResp); err != nil {
				// try to unmarshal as error msg
				var serverErrorResp requests.ServerSubErorrMsg
				if innerErr := json.Unmarshal(msg, &serverErrorResp); innerErr != nil {
					return
				}
				resCh <- &requests.Response{
					Errors: serverErrorResp.Payload,
				}
				continue
			}

			switch serverResp.Type {
			case requests.SubComplete,
				requests.SubConnectionError,
				requests.SubConnectionTerminate,
				requests.SubError:
				return
			case requests.SubData:
				resCh <- serverResp.Payload
			}
		}
	}()

	if err := <-errCh; err != nil {
		return err
	}

	return nil
}
