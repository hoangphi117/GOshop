package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"goshop/services/identity/internal/db/sqlc"
	"goshop/services/identity/internal/repository"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/messaging"
)

type mockIdentityUserRepo struct {
	repository.UserRepository
	getByEmailFn     func(ctx context.Context, email string) (sqlc.User, error)
	getByUUIDFn      func(ctx context.Context, id uuid.UUID) (sqlc.User, error)
	getAllV2Fn       func(ctx context.Context, search, orderBy, sort string, limit, offset int32, level, status *int32, deleted bool) ([]sqlc.User, error)
	countUsersFn     func(ctx context.Context, search string, level, status *int32, deleted bool) (int64, error)
	createFn         func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	updateFn         func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	updatePasswordFn func(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
	softDeleteFn     func(ctx context.Context, id uuid.UUID) (sqlc.User, error)
	restoreFn        func(ctx context.Context, id uuid.UUID) (sqlc.User, error)
	trashFn          func(ctx context.Context, id uuid.UUID) (sqlc.User, error)
}

func (m *mockIdentityUserRepo) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return sqlc.User{}, pgx.ErrNoRows
}

func (m *mockIdentityUserRepo) GetByUUID(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	if m.getByUUIDFn != nil {
		return m.getByUUIDFn(ctx, id)
	}
	return sqlc.User{}, pgx.ErrNoRows
}

func (m *mockIdentityUserRepo) GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, level, status *int32, deleted bool) ([]sqlc.User, error) {
	if m.getAllV2Fn != nil {
		return m.getAllV2Fn(ctx, search, orderBy, sort, limit, offset, level, status, deleted)
	}
	return nil, nil
}

func (m *mockIdentityUserRepo) CountUsers(ctx context.Context, search string, level, status *int32, deleted bool) (int64, error) {
	if m.countUsersFn != nil {
		return m.countUsersFn(ctx, search, level, status, deleted)
	}
	return 0, nil
}

func (m *mockIdentityUserRepo) Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
	if m.createFn != nil {
		return m.createFn(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *mockIdentityUserRepo) Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *mockIdentityUserRepo) UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
	if m.updatePasswordFn != nil {
		return m.updatePasswordFn(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *mockIdentityUserRepo) SoftDelete(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return sqlc.User{}, nil
}

func (m *mockIdentityUserRepo) Restore(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, id)
	}
	return sqlc.User{}, nil
}

func (m *mockIdentityUserRepo) Trash(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	if m.trashFn != nil {
		return m.trashFn(ctx, id)
	}
	return sqlc.User{}, nil
}

type mockTokenService struct {
	auth.TokenService
	genRefreshFn       func(user auth.UserTokenPayload) (auth.RefreshToken, error)
	storeRefreshFn     func(token auth.RefreshToken) error
	validateRefreshFn  func(token string) (auth.RefreshToken, error)
	revokeRefreshFn    func(token string) error
	rotateRefreshFn    func(oldToken string) (string, string, error)
	revokeUserSessions func(userUUID string) error
	getFamilyIDFn      func(rawToken string) (string, error)
	genAccessFamilyFn  func(user auth.UserTokenPayload, familyID string) (string, error)
	parseTokenFn       func(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	revokedUsers       []string
	revokedTokens      []string
}

func (m *mockTokenService) ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	if m.parseTokenFn != nil {
		return m.parseTokenFn(tokenString)
	}
	return nil, nil, errors.New("not configured")
}

func (m *mockTokenService) GenerateRefreshToken(user auth.UserTokenPayload) (auth.RefreshToken, error) {
	if m.genRefreshFn != nil {
		return m.genRefreshFn(user)
	}
	return auth.RefreshToken{Token: "refresh-token-1", UserUUID: user.UserUUID}, nil
}

func (m *mockTokenService) StoreRefreshToken(token auth.RefreshToken) error {
	if m.storeRefreshFn != nil {
		return m.storeRefreshFn(token)
	}
	return nil
}

func (m *mockTokenService) ValidateRefreshToken(token string) (auth.RefreshToken, error) {
	if m.validateRefreshFn != nil {
		return m.validateRefreshFn(token)
	}
	return auth.RefreshToken{Token: token}, nil
}

func (m *mockTokenService) RevokeRefreshToken(token string) error {
	m.revokedTokens = append(m.revokedTokens, token)
	if m.revokeRefreshFn != nil {
		return m.revokeRefreshFn(token)
	}
	return nil
}

func (m *mockTokenService) RotateRefreshToken(oldToken string) (string, string, error) {
	if m.rotateRefreshFn != nil {
		return m.rotateRefreshFn(oldToken)
	}
	return "", "", errors.New("not configured")
}

func (m *mockTokenService) RevokeUserSessions(userUUID string) error {
	m.revokedUsers = append(m.revokedUsers, userUUID)
	if m.revokeUserSessions != nil {
		return m.revokeUserSessions(userUUID)
	}
	return nil
}

func (m *mockTokenService) GetTokenFamilyID(rawToken string) (string, error) {
	if m.getFamilyIDFn != nil {
		return m.getFamilyIDFn(rawToken)
	}
	return "fam-1", nil
}

func (m *mockTokenService) GenerateAccessTokenWithFamily(user auth.UserTokenPayload, familyID string) (string, error) {
	if m.genAccessFamilyFn != nil {
		return m.genAccessFamilyFn(user, familyID)
	}
	return "access-token-1", nil
}

type memCache struct {
	cache.RedisCacheService
	mu   sync.Mutex
	data map[string][]byte
}

func newMemCache() *memCache {
	return &memCache{data: make(map[string][]byte)}
}

func (c *memCache) Set(key string, value any, _ time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.data[key] = b
	c.mu.Unlock()
	return nil
}

func (c *memCache) Get(key string, dest any) error {
	c.mu.Lock()
	b, ok := c.data[key]
	c.mu.Unlock()
	if !ok {
		return redis.Nil
	}
	return json.Unmarshal(b, dest)
}

func (c *memCache) Exists(key string) (bool, error) {
	c.mu.Lock()
	_, ok := c.data[key]
	c.mu.Unlock()
	return ok, nil
}

func (c *memCache) Del(key string) error {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
	return nil
}

func (c *memCache) Clear(_ string) error {
	c.mu.Lock()
	c.data = make(map[string][]byte)
	c.mu.Unlock()
	return nil
}

type mockIdentityBroker struct {
	messaging.Broker
	published []string
	err       error
}

func (b *mockIdentityBroker) Publish(_ context.Context, queue string, _ any) error {
	b.published = append(b.published, queue)
	return b.err
}

type mockOutboxRepo struct {
	repository.OutboxRepository
	inserted []string
	marked   []int64
}

func (o *mockOutboxRepo) Insert(_ context.Context, _ pgx.Tx, _, _, eventType string, _ any) (sqlc.OutboxEvent, error) {
	o.inserted = append(o.inserted, eventType)
	return sqlc.OutboxEvent{EventID: int64(len(o.inserted))}, nil
}

func (o *mockOutboxRepo) MarkProcessed(_ context.Context, ids []int64) error {
	o.marked = append(o.marked, ids...)
	return nil
}
