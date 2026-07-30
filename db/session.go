package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const DefaultIdleTTL = 30 * time.Minute

type Store struct {
	mu       sync.Mutex
	sessions map[string]*liveSession
	ttl      time.Duration
	stop     chan struct{}
}

type liveSession struct {
	Session
	lastUsed time.Time
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = DefaultIdleTTL
	}
	s := &Store{
		sessions: make(map[string]*liveSession),
		ttl:      ttl,
		stop:     make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *Store) Close() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		_ = sess.DB.Close()
		delete(s.sessions, id)
	}
}

func (s *Store) Put(sess *Session) string {
	id := newID()
	sess.ID = id
	s.mu.Lock()
	s.sessions[id] = &liveSession{Session: *sess, lastUsed: time.Now()}
	s.mu.Unlock()
	return id
}

// Get returns the live session (DB pointer is shared). Touches lastUsed.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("db session not found")
	}
	live.lastUsed = time.Now()
	return &live.Session, nil
}

func (s *Store) UpdateDatabase(id, database string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("db session not found")
	}
	live.Database = database
	live.lastUsed = time.Now()
	return nil
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	live, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("db session not found")
	}
	err := live.DB.Close()
	delete(s.sessions, id)
	return err
}

func (s *Store) cleanupLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.expire()
		}
	}
}

func (s *Store) expire() {
	cutoff := time.Now().Add(-s.ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, live := range s.sessions {
		if live.lastUsed.Before(cutoff) {
			_ = live.DB.Close()
			delete(s.sessions, id)
		}
	}
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
