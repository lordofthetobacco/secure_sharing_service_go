package store

import (
	"context"
	"sync"
	"time"

	"secure.share/internal/models"
)

// Compile-time interface check
var _ Store = (*MemoryStore)(nil)

type MemoryStore struct {
	secrets       map[string]*models.Secret
	mu            sync.RWMutex
	cleanupCancel context.CancelFunc
}

func NewMemoryStore(cleanupInterval time.Duration) *MemoryStore {
	ctx, cancel := context.WithCancel(context.Background())
	store := &MemoryStore{
		secrets:       make(map[string]*models.Secret),
		cleanupCancel: cancel,
	}
	go store.cleanupLoop(ctx, cleanupInterval)
	return store
}

func (s *MemoryStore) Save(ctx context.Context, secret *models.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secrets[secret.ID] = secret
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*models.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	secret, ok := s.secrets[id]
	if !ok {
		return nil, ErrNotFound
	}

	// Check expiry
	if time.Now().After(secret.ExpiresAt) {
		return nil, ErrExpired
	}

	// Check max views
	if secret.CurrentViews >= secret.MaxViews {
		return nil, ErrMaxViews
	}

	return secret, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.secrets, id)
	return nil
}

func (s *MemoryStore) IncrementViews(ctx context.Context, id string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	secret, ok := s.secrets[id]
	if !ok {
		return 0, ErrNotFound
	}

	if time.Now().After(secret.ExpiresAt) {
		delete(s.secrets, id)
		return 0, ErrExpired
	}

	if secret.CurrentViews >= secret.MaxViews {
		delete(s.secrets, id)
		return 0, ErrMaxViews
	}

	secret.CurrentViews++

	// Auto-delete if max views reached
	if secret.CurrentViews >= secret.MaxViews {
		delete(s.secrets, id)
	}

	return secret.CurrentViews, nil
}

func (s *MemoryStore) Close() error {
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secrets = nil
	return nil
}

func (s *MemoryStore) cleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *MemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, secret := range s.secrets {
		if now.After(secret.ExpiresAt) || secret.CurrentViews >= secret.MaxViews {
			delete(s.secrets, id)
		}
	}
}
