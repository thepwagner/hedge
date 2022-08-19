package cache

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"time"
)

type TrustedStorage struct {
	trustedKeys map[string][]byte
	activeKey   string
	storage     Storage
}

var _ Storage = (*TrustedStorage)(nil)

func NewTrustedStorage(keys map[string][]byte, activeKey string, storage Storage) TrustedStorage {
	return TrustedStorage{
		trustedKeys: keys,
		activeKey:   activeKey,
		storage:     storage,
	}
}

type SignedEntry struct {
	Key       string `json:"key"`
	Signature []byte `json:"signature"`
	Value     []byte `json:"payload"`
}

func (c TrustedStorage) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := c.storage.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var e SignedEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, err
	}

	hmacKey, ok := c.trustedKeys[e.Key]
	if !ok {
		return nil, errors.New("invalid key")
	}
	h := hmac.New(sha256.New, hmacKey).Sum(e.Value)
	if subtle.ConstantTimeCompare(h, e.Signature) != 1 {
		return nil, errors.New("invalid signature")
	}
	return e.Value, nil
}

func (c TrustedStorage) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	keyID := c.activeKey
	hmacKey := c.trustedKeys[keyID]
	signature := hmac.New(sha256.New, hmacKey).Sum(value)
	e := SignedEntry{
		Key:       keyID,
		Signature: signature,
		Value:     value,
	}
	be, err := json.Marshal(&e)
	if err != nil {
		return err
	}
	return c.storage.Set(ctx, key, be, ttl)
}
