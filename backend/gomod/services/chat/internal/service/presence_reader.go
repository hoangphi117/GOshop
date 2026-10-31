package service

import (
	"context"

	ws "goshop/shared/pkg/ws"
)

// RedisPresenceReader adapts the shared Redis presence ZSET (owned and written
// by the WS gateway) to the PresenceReader port declared in ports.go.
type RedisPresenceReader struct {
	store *ws.PresenceStore
}

func NewRedisPresenceReader(store *ws.PresenceStore) *RedisPresenceReader {
	return &RedisPresenceReader{store: store}
}

func (r *RedisPresenceReader) IsUserOnline(ctx context.Context, userID int32) bool {
	return r.store.IsUserOnline(ctx, userID)
}

func (r *RedisPresenceReader) HasOnlineAdmin(ctx context.Context) bool {
	return r.store.HasOnlineAdmin(ctx)
}
