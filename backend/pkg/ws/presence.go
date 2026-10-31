package ws

import (
	"context"
	"strconv"
	"strings"
	"time"

	"goshop/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// PresenceKey is the Redis ZSET holding live WebSocket connections across all
// gateway instances. Members are "<user_id>|<is_admin>", scores are unix expiry
// timestamps, so crashed processes self-heal once their entries expire.
//
// ponytail: one member per user identity, not per socket. Multiple sockets for
// the same user share the member and may drop presence early on one close
// (refreshed on the next ping). Split per connection when multi-tab presence
// precision matters.
const PresenceKey = "ws:presence:conns"

// PresenceTTL must outlive the client pong wait (60s) so live connections are
// never reaped, while dead ones disappear within TTL.
const PresenceTTL = 75 * time.Second

// PresenceStore maintains the presence ZSET on behalf of a hub instance.
type PresenceStore struct {
	rdb *redis.Client
}

func NewPresenceStore(rdb *redis.Client) *PresenceStore {
	return &PresenceStore{rdb: rdb}
}

func presenceMember(c *Client) string {
	return strconv.FormatInt(int64(c.UserID), 10) + "|" + strconv.FormatBool(c.IsAdmin)
}

// Touch records (or refreshes) the connection with a new expiry score.
func (s *PresenceStore) Touch(ctx context.Context, c *Client) {
	if s.rdb == nil {
		return
	}
	score := float64(time.Now().Add(PresenceTTL).Unix())
	if err := s.rdb.ZAdd(ctx, PresenceKey, redis.Z{Score: score, Member: presenceMember(c)}).Err(); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to record presence in Redis")
	}
}

// Remove drops the connection entry (best-effort).
func (s *PresenceStore) Remove(ctx context.Context, c *Client) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.ZRem(ctx, PresenceKey, presenceMember(c)).Err(); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to remove presence from Redis")
	}
}

func (s *PresenceStore) liveMembers(ctx context.Context) ([]string, error) {
	now := strconv.FormatInt(time.Now().Unix(), 10)
	if err := s.rdb.ZRemRangeByScore(ctx, PresenceKey, "-inf", now).Err(); err != nil {
		return nil, err
	}
	return s.rdb.ZRange(ctx, PresenceKey, 0, -1).Result()
}

// IsUserOnline reports whether the user has a live connection.
func (s *PresenceStore) IsUserOnline(ctx context.Context, userID int32) bool {
	if s.rdb == nil {
		return false
	}
	members, err := s.liveMembers(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to read presence from Redis")
		return false
	}
	prefix := strconv.FormatInt(int64(userID), 10) + "|"
	for _, m := range members {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

// HasOnlineAdmin reports whether any admin/moderator connection is live.
func (s *PresenceStore) HasOnlineAdmin(ctx context.Context) bool {
	if s.rdb == nil {
		return false
	}
	members, err := s.liveMembers(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to read presence from Redis")
		return false
	}
	for _, m := range members {
		parts := strings.SplitN(m, "|", 2)
		if len(parts) == 2 && parts[1] == "true" {
			return true
		}
	}
	return false
}
