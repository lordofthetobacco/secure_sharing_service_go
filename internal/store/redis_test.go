package store

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"secure.share/internal/models"
)

func TestRedisStore(t *testing.T) {
	store, err := NewRedisStore(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err != nil {
		t.Fatalf("failed to create redis store: %v", err)
	}
	secret := &models.Secret{
		ID:            "123",
		EncryptedData: []byte("test"),
		MaxViews:      1,
		CurrentViews:  0,
		ExpiresAt:     time.Now().Add(1 * time.Hour),
		CreatedAt:     time.Now(),
		Passphrase:    "test",
	}
	dead_secret := &models.Secret{
		ID:            "1234",
		EncryptedData: []byte("test4"),
		MaxViews:      1,
		CurrentViews:  0,
		ExpiresAt:     time.Now().Add(-1 * time.Hour),
		CreatedAt:     time.Now(),
		Passphrase:    "test",
	}
	store.Save(context.Background(), secret)
	store.Save(context.Background(), dead_secret)
	t.Log("Starting timeout")
	time.Sleep(2 * time.Second)
	t.Log("After timeout")
	secret, err = store.Get(context.Background(), secret.ID)
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}
	if secret.ID != "123" {
		t.Fatalf("secret ID mismatch: got %s, want %s", secret.ID, "123")
	}
	if string(secret.EncryptedData) != "test" {
		t.Fatalf("secret data mismatch: got %s, want %s", string(secret.EncryptedData), "test")
	}
	secret, err = store.Get(context.Background(), dead_secret.ID)
	if err != nil {
		t.Logf("failed to get secret: %v", err)
	}
	if secret != nil {
		t.Fatalf("secret should be nil")
	}
}
