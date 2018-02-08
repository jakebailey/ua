package expire

import (
	"sync/atomic"
	"time"
)

// Token keep tracks of the last time it was used. The expiry manager uses
// checks for tokens that haven't been updated, and expires them.
type Token struct {
	unix int64
}

func newToken() *Token {
	return &Token{
		unix: time.Now().Unix(),
	}
}

// Update updates the internal timestamp of the token. This should be called
// to keep the token up to date.
//
// Update is safe for concurrent use.
func (t *Token) Update() {
	atomic.StoreInt64(&t.unix, time.Now().Unix())
}

func (t *Token) after(u time.Time) bool {
	return atomic.LoadInt64(&t.unix) > u.Unix()
}
