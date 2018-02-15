package proxy

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/jakebailey/ua/pkg/errhack"
)

// WSConn wraps a gobwas/ws connection.
type WSConn struct {
	c  net.Conn
	mu sync.Mutex
}

var _ Conn = (*WSConn)(nil)

// NewWSConn creates a new WSConn from a net.Conn.
func NewWSConn(conn net.Conn) *WSConn {
	return &WSConn{c: conn}
}

// ReadJSON parses the next text websocket text message into JSON.
func (w *WSConn) ReadJSON(v interface{}) error {
	buf, err := wsutil.ReadClientText(w.c)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, v)
}

// WriteJSON writes a value to the websocket as JSON. It is safe for
// concurrent use.
func (w *WSConn) WriteJSON(v interface{}) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}

	w.mu.Lock()
	err = wsutil.WriteServerText(w.c, buf)
	w.mu.Unlock()
	return err
}

// Close closes the connection.
func (w *WSConn) Close() error {
	return w.c.Close()
}

// IsClose returns true if the error provided is a normal closure error
// and can be ignored. This includes io.EOF and a wsutil.ClosedError
// with the code set to StatusNormalClosure or StatusGoingAway.
func (w *WSConn) IsClose(err error) bool {
	if errhack.IsClose(err) {
		return true
	}

	if cErr, ok := err.(wsutil.ClosedError); ok {
		switch cErr.Code() {
		case ws.StatusNormalClosure, ws.StatusGoingAway:
			return true
		}
	}

	return false
}
