// redis.go
package store

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"time"

	"bx.share/internal/models"
	"github.com/redis/go-redis/v9"
)

var _ Store = (*RedisStore)(nil)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(options *redis.Options) (*RedisStore, error) {
	client := redis.NewClient(options)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStore{client: client}, nil
}

func (r *RedisStore) Save(ctx context.Context, secret *models.Secret) error {
	data, err := encode(secret)
	if err != nil {
		return err
	}

	ttl := time.Until(secret.ExpiresAt)
	if ttl <= 0 {
		return ErrExpired
	}

	return r.client.Set(ctx, secretKey(secret.ID), data, ttl).Err()
}

func (r *RedisStore) Get(ctx context.Context, id string) (*models.Secret, error) {
	data, err := r.client.Get(ctx, secretKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	secret, err := decode(data)
	if err != nil {
		return nil, err
	}

	// Check max views
	if secret.CurrentViews >= secret.MaxViews {
		_ = r.Delete(ctx, id)
		return nil, ErrMaxViews
	}

	return secret, nil
}

func (r *RedisStore) Delete(ctx context.Context, id string) error {
	return r.client.Del(ctx, secretKey(id)).Err()
}

var incrementViewsScript = redis.NewScript(`
	local key = KEYS[1]
	local data = redis.call('GET', key)
	if not data then
		return nil
	end
	return data
`)

func (r *RedisStore) IncrementViews(ctx context.Context, id string) (int, error) {
	key := secretKey(id)
	var resultViews int

	txf := func(tx *redis.Tx) error {
		result := incrementViewsScript.Run(ctx, tx, []string{key})
		if result.Err() != nil {
			if errors.Is(result.Err(), redis.Nil) {
				return ErrNotFound
			}
			return result.Err()
		}

		val, err := result.Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return ErrNotFound
			}
			return err
		}

		var data []byte
		switch v := val.(type) {
		case string:
			data = []byte(v)
		case []byte:
			data = v
		default:
			return errors.New("unexpected data type from script")
		}

		secret, err := decode(data)
		if err != nil {
			return err
		}

		now := time.Now()
		if now.After(secret.ExpiresAt) {
			return ErrExpired
		}

		if secret.CurrentViews >= secret.MaxViews {
			return ErrMaxViews
		}

		secret.CurrentViews++
		resultViews = secret.CurrentViews

		newData, err := encode(secret)
		if err != nil {
			return err
		}

		ttl := tx.TTL(ctx, key).Val()
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if secret.CurrentViews >= secret.MaxViews {
				pipe.Del(ctx, key)
			} else if ttl > 0 {
				pipe.Set(ctx, key, newData, ttl)
			}
			return nil
		})
		return err
	}

	for i := 0; i < 3; i++ {
		err := r.client.Watch(ctx, txf, key)
		if err == nil {
			return resultViews, nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if errors.Is(err, ErrExpired) || errors.Is(err, ErrMaxViews) {
			_ = r.Delete(ctx, id)
			return 0, err
		}
		if errors.Is(err, ErrNotFound) {
			return 0, err
		}
		return 0, err
	}

	return 0, redis.TxFailedErr
}

func (r *RedisStore) Close() error {
	return r.client.Close()
}

// Helpers

func secretKey(id string) string {
	return "secret:" + id
}

func encode(secret *models.Secret) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(secret); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode(data []byte) (*models.Secret, error) {
	var secret models.Secret
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&secret); err != nil {
		return nil, err
	}
	return &secret, nil
}
