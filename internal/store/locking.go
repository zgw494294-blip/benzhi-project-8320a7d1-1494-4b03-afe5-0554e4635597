package store

import "sync"

type KeyLocker struct {
	mu   sync.Mutex
	keys map[string]*sync.Mutex
}

func NewKeyLocker() *KeyLocker { return &KeyLocker{keys: map[string]*sync.Mutex{}} }
func (l *KeyLocker) Lock(key string) func() {
	l.mu.Lock()
	k := l.keys[key]
	if k == nil {
		k = &sync.Mutex{}
		l.keys[key] = k
	}
	l.mu.Unlock()
	k.Lock()
	return k.Unlock
}
