package proxy

import (
	"encoding/json"
	"io"
	"net"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WSConn struct {
	c  net.Conn
	mu sync.Mutex
}

var _ Conn = (*WSConn)(nil)

func NewWSConn(conn net.Conn) *WSConn {
	return &WSConn{c: conn}
}

func (w *WSConn) ReadJSON(v interface{}) error {
	buf, err := wsutil.ReadClientText(w.c)
	if err != nil {
		return err
	}

	return json.Unmarshal(buf, v)
}

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

func (w *WSConn) Close() error {
	return w.c.Close()
}

func (w *WSConn) IsClose(err error) bool {
	if err == io.EOF {
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
