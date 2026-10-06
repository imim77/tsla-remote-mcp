package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestInMemoryStoreLoadBeforeSave(t *testing.T) {
	_, err := NewInMemoryStore().Load(context.Background())
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("Load() error = %v, want ErrTokenNotFound", err)
	}
}

func TestInMemoryStoreSaveAndLoad(t *testing.T) {
	store := NewInMemoryStore()
	want := &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := store.Save(context.Background(), want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.TokenType != want.TokenType || !got.Expiry.Equal(want.Expiry) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestInMemoryStoreReturnsCopy(t *testing.T) {
	store := NewInMemoryStore()
	original := &oauth2.Token{AccessToken: "original"}
	if err := store.Save(context.Background(), original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	original.AccessToken = "changed after save"

	loaded, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	loaded.AccessToken = "changed after load"

	again, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if again.AccessToken != "original" {
		t.Fatalf("Load().AccessToken = %q, want %q", again.AccessToken, "original")
	}
}
