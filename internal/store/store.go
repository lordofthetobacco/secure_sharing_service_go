package store

import (
	"context"
	"errors"

	"secure.share/internal/models"
)

var (
	ErrNotFound = errors.New("secret not found")
	ErrExpired  = errors.New("secret has expired")
	ErrMaxViews = errors.New("secret has reached maximum views")
)

type Store interface {
	Save(ctx context.Context, secret *models.Secret) error
	Get(ctx context.Context, id string) (*models.Secret, error)
	Delete(ctx context.Context, id string) error
	IncrementViews(ctx context.Context, id string) (currentViews int, err error)
	Close() error
}
