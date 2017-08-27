package expire

import (
	"sync"
	"time"
)

type entry struct {
	token    *Token
	onExpire func()
}

// Manager manages expiration. Periodically, the manager will check its tokens
// for those that are past their expiration limit. When found, the token's
// registered onExpire function will be run in a new goroutine (so that the
// function can call the delete function without deadlocking).
type Manager struct {
	checkEvery  time.Duration
	expireLimit time.Duration

	mu       sync.RWMutex
	entries  map[string]*entry
	tokens   map[*Token]string
	blockers map[*Token]chan struct{}

	done chan struct{}
}

// NewManager creates a new manager that checks for expired entries every
// checkEvery duration and expires those that are expireLimit or older.
func NewManager(checkEvery time.Duration, expireLimit time.Duration) *Manager {
	return &Manager{
		checkEvery:  checkEvery,
		expireLimit: expireLimit,
		entries:     make(map[string]*entry),
		tokens:      make(map[*Token]string),
		blockers:    make(map[*Token]chan struct{}),
		done:        make(chan struct{}),
	}
}

// Run starts the manager.
func (m *Manager) Run() {
	go m.run()
}

// Stop stops the manager.
func (m *Manager) Stop() {
	m.done <- struct{}{}
}

// Acquire acquires a token for the given name. If the specified name already
// has an entry in the manager, then that existing entry will be expired and
// its onExpire function run. The new entry replaces it. A Token is returned,
// which must be updated in order to not be expired.
//
// Acquire is safe for concurrent use.
func (m *Manager) Acquire(name string, onExpire func()) *Token {
	token := newToken()
	e := &entry{
		token:    token,
		onExpire: onExpire,
	}

	var blocker chan struct{}

	m.mu.Lock()

	if old := m.entries[name]; old != nil {
		blocker = make(chan struct{})
		m.blockers[old.token] = blocker

		delete(m.tokens, old.token)
		go old.onExpire()
	}
	m.entries[name] = e
	m.tokens[token] = name

	m.mu.Unlock()

	if blocker != nil {
		<-blocker
	}

	return token
}

// Return returns a token to the manager, deregistering it. This will not
// run the associated onExpire function. If the given token is not being
// tracked, Return does nothing.
//
// Return is safe for concurrent use.
func (m *Manager) Return(token *Token) {
	m.mu.Lock()

	if name, ok := m.tokens[token]; ok {
		delete(m.entries, name)
		delete(m.tokens, token)
	}

	if blocker, ok := m.blockers[token]; ok {
		close(blocker)
		delete(m.blockers, token)
	}

	m.mu.Unlock()
}

// ExpireAndRemove expires an entry in the manager by name, removing it
// and running its onExpire function.
func (m *Manager) ExpireAndRemove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry := m.entries[name]; entry != nil {
		delete(m.entries, name)
		delete(m.tokens, entry.token)
		go entry.onExpire()
	}
}

// ExpireAndRemoveAll expires all entries and removes them from the manager,
// running onExpire functions.
func (m *Manager) ExpireAndRemoveAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, entry := range m.entries {
		delete(m.entries, name)
		delete(m.tokens, entry.token)
		go entry.onExpire()
	}
}

func (m *Manager) run() {
	ticker := time.NewTicker(m.checkEvery)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case t := <-ticker.C:
			t = t.Add(-m.expireLimit)
			m.expire(t)
		}
	}
}

func (m *Manager) expire(threshold time.Time) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, entry := range m.entries {
		if !entry.token.after(threshold) {
			go entry.onExpire()
		}
	}
}
