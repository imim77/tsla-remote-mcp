package store

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/oauth2"
)

var ErrTokenNotFound = errors.New("token not found")

type Repository interface {
	Save(ctx context.Context, token *oauth2.Token) error
	Load(ctx context.Context) (*oauth2.Token, error)
}

type InMemoryStore struct {
	mu    sync.RWMutex
	token *oauth2.Token
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

func (in *InMemoryStore) Save(ctx context.Context, token *oauth2.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if token == nil {
		return errors.New("cannot save a nil token")
	}

	copyToken := *token
	in.mu.Lock()
	in.token = &copyToken
	in.mu.Unlock()
	return nil
}

func (in *InMemoryStore) Load(ctx context.Context) (*oauth2.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	in.mu.RLock()
	defer in.mu.RUnlock()
	if in.token == nil {
		return nil, ErrTokenNotFound
	}

	copyToken := *in.token
	return &copyToken, nil
}
