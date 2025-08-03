package secret

import "fmt"

var ErrSecretNotFound = fmt.Errorf("secret not found")

type Store interface {
	// Get retrieves a secret by its key.
	Get(key string) (string, error)
	// Set stores a secret with the given key and value.
	Set(key, value string) error

	Close() error
}

type InMemoryStore struct {
	secrets map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		secrets: make(map[string]string),
	}
}

func (s *InMemoryStore) Get(key string) (string, error) {
	value, exists := s.secrets[key]
	if !exists {
		return "", ErrSecretNotFound
	}
	return value, nil
}

func (s *InMemoryStore) Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}
	s.secrets[key] = value
	return nil
}

func (s *InMemoryStore) Close() error {
	if len(s.secrets) > 0 {
		s.secrets = make(map[string]string) // Clear secrets on close
	}
	return nil
}
