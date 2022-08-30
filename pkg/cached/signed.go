package cached

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/thepwagner/hedge/proto/hedge/v1"
	"google.golang.org/protobuf/proto"
)

type SignedCache struct {
	trustedKeys map[string][]byte
	activeKey   string
	cache       Cache[string, []byte]
}

var _ Cache[string, []byte] = (*SignedCache)(nil)

func NewSignedCache(keys map[string][]byte, activeKey string, storage Cache[string, []byte]) SignedCache {
	return SignedCache{
		trustedKeys: keys,
		activeKey:   activeKey,
		cache:       storage,
	}
}

func (c SignedCache) Get(ctx context.Context, key string) (*[]byte, error) {
	b, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}

	var e hedge.SignedEntry
	if err := proto.Unmarshal(*b, &e); err != nil {
		return nil, err
	}

	hmacKey, ok := c.trustedKeys[e.GetKeyId()]
	if !ok {
		return nil, errors.New("invalid key")
	}
	payload := e.GetPayload()
	h := hmac.New(sha256.New, hmacKey).Sum(payload)
	if subtle.ConstantTimeCompare(h, e.Signature) != 1 {
		return nil, errors.New("invalid signature")
	}
	return &payload, nil
}

func (c SignedCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	keyID := c.activeKey
	hmacKey := c.trustedKeys[keyID]
	signature := hmac.New(sha256.New, hmacKey).Sum(value)
	e := hedge.SignedEntry{
		KeyId:     keyID,
		Signature: signature,
		Payload:   value,
	}
	be, err := proto.Marshal(&e)
	if err != nil {
		return err
	}
	return c.cache.Set(ctx, key, be, ttl)
}
