package xrpld

import (
	"sync"
)

// syncMap is a generic thread-safe map. It is safe for zero-value usage.
type syncMap[A comparable, B any] struct {
	data  map[A]B
	mutex sync.RWMutex
	once  sync.Once
}

// set a value in the map.
func (s *syncMap[A, B]) set(key A, value B) {
	s.initializeOnce()

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[key] = value
}

// get a value from the map.
func (s *syncMap[A, B]) get(key A) (B, bool) {
	s.initializeOnce()

	s.mutex.RLock()
	defer s.mutex.RUnlock()
	value, exists := s.data[key]
	return value, exists
}

// initializeOnce ensures the syncMap is safe for zero-value usage.
func (s *syncMap[A, B]) initializeOnce() {
	s.once.Do(func() { s.data = map[A]B{} })
}
