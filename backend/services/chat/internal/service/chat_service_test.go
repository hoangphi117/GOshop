package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgpgx "goshop/pkg/pgx"
	ws "goshop/pkg/ws"
	sqlc "goshop/services/chat/internal/db/sqlc"
	chatdto "goshop/services/chat/internal/dto"
	chatservice "goshop/services/chat/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// --- local test doubles ---

type mockChatRepo struct {
	conversations map[int32]sqlc.ChatConversation
	byUserID      map[int32]sqlc.ChatConversation
	messages      map[uuid.UUID]sqlc.ChatMessage
	createdMsgs   []sqlc.CreateMessageParams
	listRows      []sqlc.ListConversationsRow
}

func newMockChatRepo() *mockChatRepo {
	return &mockChatRepo{
		conversations: map[int32]sqlc.ChatConversation{},
		byUserID:      map[int32]sqlc.ChatConversation{},
		messages:      map[uuid.UUID]sqlc.ChatMessage{},
	}
}

func (m *mockChatRepo) CreateConversation(_ context.Context, p sqlc.CreateConversationParams) (sqlc.ChatConversation, error) {
	conv := sqlc.ChatConversation{
		ConversationID:   int32(len(m.conversations) + 1),
		ConversationUuid: uuid.New(),
		UserID:           p.UserID,
		UserEmail:        p.UserEmail,
		UserFullname:     p.UserFullname,
		UserAvatar:       p.UserAvatar,
		Status:           "open",
	}
	m.conversations[conv.ConversationID] = conv
	m.byUserID[p.UserID] = conv
	return conv, nil
}

func (m *mockChatRepo) GetConversationByUUID(_ context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
	for _, c := range m.conversations {
		if c.ConversationUuid == u {
			return c, nil
		}
	}
	return sqlc.ChatConversation{}, pgx.ErrNoRows
}

func (m *mockChatRepo) GetConversationByID(_ context.Context, id int32) (sqlc.ChatConversation, error) {
	if c, ok := m.conversations[id]; ok {
		return c, nil
	}
	return sqlc.ChatConversation{}, pgx.ErrNoRows
}

func (m *mockChatRepo) GetConversationByUserID(_ context.Context, userID int32) (sqlc.ChatConversation, error) {
	if c, ok := m.byUserID[userID]; ok {
		return c, nil
	}
	return sqlc.ChatConversation{}, pgx.ErrNoRows
}

func (m *mockChatRepo) UpdateConversationStatus(_ context.Context, id int32, status string) (sqlc.ChatConversation, error) {
	c := m.conversations[id]
	c.Status = status
	m.conversations[id] = c
	return c, nil
}

func (m *mockChatRepo) UpdateLastMessageAt(_ context.Context, _ int32) error { return nil }

func (m *mockChatRepo) UpdateConversationUserInfo(_ context.Context, p sqlc.UpdateConversationUserInfoParams) error {
	c := m.conversations[p.ConversationID]
	c.UserEmail = p.UserEmail
	c.UserFullname = p.UserFullname
	c.UserAvatar = p.UserAvatar
	m.conversations[p.ConversationID] = c
	m.byUserID[c.UserID] = c
	return nil
}

func (m *mockChatRepo) ListConversations(_ context.Context, _ sqlc.ListConversationsParams) ([]sqlc.ListConversationsRow, error) {
	return m.listRows, nil
}

func (m *mockChatRepo) CountConversations(_ context.Context, _ sqlc.CountConversationsParams) (int64, error) {
	return 0, nil
}

func (m *mockChatRepo) CreateMessage(_ context.Context, p sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
	msg := sqlc.ChatMessage{
		MessageID:      int32(len(m.createdMsgs) + 1),
		MessageUuid:    uuid.New(),
		ConversationID: p.ConversationID,
		SenderID:       p.SenderID,
		SenderRole:     p.SenderRole,
		Content:        p.Content,
		ContentType:    p.ContentType,
		AttachmentUrl:  p.AttachmentUrl,
		AttachmentName: p.AttachmentName,
		AttachmentSize: p.AttachmentSize,
		CreatedAt:      time.Now(),
	}
	m.createdMsgs = append(m.createdMsgs, p)
	m.messages[msg.MessageUuid] = msg
	return msg, nil
}

func (m *mockChatRepo) ListMessages(_ context.Context, _ sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error) {
	return nil, nil
}

func (m *mockChatRepo) ListMessagesCursor(_ context.Context, _ sqlc.ListMessagesCursorParams) ([]sqlc.ChatMessage, error) {
	return nil, nil
}

func (m *mockChatRepo) CountMessagesByConversationID(_ context.Context, _ int32) (int64, error) {
	return 0, nil
}

func (m *mockChatRepo) MarkMessagesRead(_ context.Context, _, _ int32) error { return nil }

func (m *mockChatRepo) CountUnreadMessages(_ context.Context, _, _ int32) (int64, error) {
	return 0, nil
}

func (m *mockChatRepo) GetTotalUnreadMessagesForUser(_ context.Context, _ int32) (int64, error) {
	return 0, nil
}

func (m *mockChatRepo) GetTotalUnreadMessagesForAdmin(_ context.Context) (int64, error) {
	return 0, nil
}

func (m *mockChatRepo) GetMessageByUUID(_ context.Context, u uuid.UUID) (sqlc.ChatMessage, error) {
	if msg, ok := m.messages[u]; ok {
		return msg, nil
	}
	return sqlc.ChatMessage{}, pgx.ErrNoRows
}

func (m *mockChatRepo) RecallMessage(_ context.Context, id int32) (sqlc.ChatMessage, error) {
	for u, msg := range m.messages {
		if msg.MessageID == id {
			msg.Content = "This message was unsent"
			msg.ContentType = "recalled"
			m.messages[u] = msg
			return msg, nil
		}
	}
	return sqlc.ChatMessage{}, pgx.ErrNoRows
}

type mockUserRepo struct {
	users map[uuid.UUID]chatservice.User
	byID  map[int32]chatservice.User
}

func (m *mockUserRepo) GetByUUID(_ context.Context, u uuid.UUID) (chatservice.User, error) {
	if usr, ok := m.users[u]; ok {
		return usr, nil
	}
	return chatservice.User{}, pgx.ErrNoRows
}

func (m *mockUserRepo) GetByID(_ context.Context, id int32) (chatservice.User, error) {
	if usr, ok := m.byID[id]; ok {
		return usr, nil
	}
	return chatservice.User{}, pgx.ErrNoRows
}

type mockOrderReader struct {
	orders map[uuid.UUID]*chatservice.OrderSummary
}

func (m *mockOrderReader) GetOrderSummary(_ context.Context, u uuid.UUID) (*chatservice.OrderSummary, error) {
	if o, ok := m.orders[u]; ok {
		return o, nil
	}
	return nil, errors.New("order not found")
}

type mockPresence struct {
	onlineUserIDs map[int32]bool
	adminOnline   bool
}

func (m *mockPresence) IsUserOnline(_ context.Context, userID int32) bool {
	return m.onlineUserIDs[userID]
}

func (m *mockPresence) HasOnlineAdmin(_ context.Context) bool { return m.adminOnline }

type mockPublisher struct {
	events chan struct {
		ConversationID int32
		Type           string
	}
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{events: make(chan struct {
		ConversationID int32
		Type           string
	}, 16)}
}

func (m *mockPublisher) PublishToConversation(_ context.Context, conversationID int32, msgType string, _ any) {
	m.events <- struct {
		ConversationID int32
		Type           string
	}{conversationID, msgType}
}

// awaitEvent reads one published event or fails the test after a grace period
// (publishing is asynchronous in the service).
func (m *mockPublisher) awaitEvent(t *testing.T) struct {
	ConversationID int32
	Type           string
} {
	t.Helper()
	select {
	case ev := <-m.events:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("expected a published event, got none within 2s")
		return struct {
			ConversationID int32
			Type           string
		}{}
	}
}

// Use the shared no-op TxManager surface through the pkgpgx interface: tests
// pass nil txManager by keeping nilCheck in NewChatService (nil branch absent).
// This local noTxManager satisfies the interface but always runs inline.
type noTxManager struct{}

func (noTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

var _ pkgpgx.TxManager = (*txRecorder)(nil)

type txRecorder struct {
	began int
}

func (t *txRecorder) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	t.began++
	return fn(ctx)
}

// --- tests ---

func newService(repo *mockChatRepo, users *mockUserRepo, orders *mockOrderReader, tx pkgpgx.TxManager) (chatservice.ChatService, *mockPublisher, *mockPresence) {
	publisher := newMockPublisher()
	presence := &mockPresence{onlineUserIDs: map[int32]bool{}}
	svc := chatservice.NewChatService(repo, users, orders, presence, publisher, tx)
	return svc, publisher, presence
}

func activeCustomer(id int32) (uuid.UUID, chatservice.User) {
	u := uuid.New()
	return u, chatservice.User{UserID: id, UserUUID: u, UserStatus: 1, UserLevel: 3, UserEmail: "u@test.dev", UserFullname: "Customer"}
}

func activeAdmin(id int32) (uuid.UUID, chatservice.User) {
	u := uuid.New()
	return u, chatservice.User{UserID: id, UserUUID: u, UserStatus: 1, UserLevel: chatservice.UserLevelAdmin, UserEmail: "a@test.dev", UserFullname: "Admin"}
}

func TestGetOrCreateConversation(t *testing.T) {
	repo := newMockChatRepo()
	users := &mockUserRepo{users: map[uuid.UUID]chatservice.User{}}
	svc, _, _ := newService(repo, users, &mockOrderReader{}, noTxManager{})

	userUUID, user := activeCustomer(10)
	users.users[userUUID] = user

	conv, err := svc.GetOrCreateConversation(context.Background(), userUUID)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), conv.ConversationID)

	again, err := svc.GetOrCreateConversation(context.Background(), userUUID)
	assert.NoError(t, err)
	assert.Equal(t, conv.ConversationID, again.ConversationID, "must reuse the open conversation")
}

func TestSendMessage_RulesAndEvents(t *testing.T) {
	repo := newMockChatRepo()
	users := &mockUserRepo{users: map[uuid.UUID]chatservice.User{}}
	orders := &mockOrderReader{orders: map[uuid.UUID]*chatservice.OrderSummary{}}

	custUUID, customer := activeCustomer(10)
	adminUUID, admin := activeAdmin(20)
	users.users[custUUID] = customer
	users.users[adminUUID] = admin

	orderUUID := uuid.New()
	orders.orders[orderUUID] = &chatservice.OrderSummary{OrderUUID: orderUUID, UserID: 10}
	strangerUUID, stranger := activeCustomer(99)
	users.users[strangerUUID] = stranger

	tx := &txRecorder{}
	svc, publisher, _ := newService(repo, users, orders, tx)

	conv, err := svc.GetOrCreateConversation(context.Background(), custUUID)
	assert.NoError(t, err)

	// Customer sends plain text: contentType inferred, event published, tx used.
	msg, err := svc.SendMessage(context.Background(), conv.ConversationUuid, custUUID, "user", chatdto.SendMessageRequest{Content: "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "text", msg.ContentType)
	assert.Equal(t, "user", msg.SenderRole)
	assert.Equal(t, 1, tx.began, "message + timestamp update must run in one tx")
	ev := publisher.awaitEvent(t)
	assert.Equal(t, "chat.message", ev.Type)
	assert.Equal(t, conv.ConversationID, ev.ConversationID)

	// Image-only message: empty content + attachment must be accepted.
	imgMsg, err := svc.SendMessage(context.Background(), conv.ConversationUuid, custUUID, "user", chatdto.SendMessageRequest{
		ContentType:    "image",
		AttachmentURL:  "https://cdn.example.com/chat/abc.png",
		AttachmentName: "abc.png",
		AttachmentSize: 1234,
	})
	assert.NoError(t, err, "attachment-only message must be allowed")
	assert.Equal(t, "image", imgMsg.ContentType)
	publisher.awaitEvent(t)

	// Neither content nor attachment: rejected.
	_, err = svc.SendMessage(context.Background(), conv.ConversationUuid, custUUID, "user", chatdto.SendMessageRequest{})
	assert.Error(t, err, "empty message must be rejected")

	// Customer references someone else's order: forbidden.
	_, err = svc.SendMessage(context.Background(), conv.ConversationUuid, custUUID, "user", chatdto.SendMessageRequest{Content: orderUUID.String(), ContentType: "order_ref", OrderUUID: orderUUID.String()})
	assert.NoError(t, err, "owner of the order may reference it")

	foreignOrder := uuid.New()
	orders.orders[foreignOrder] = &chatservice.OrderSummary{OrderUUID: foreignOrder, UserID: 777}
	_, err = svc.SendMessage(context.Background(), conv.ConversationUuid, custUUID, "user", chatdto.SendMessageRequest{Content: foreignOrder.String(), ContentType: "order_ref", OrderUUID: foreignOrder.String()})
	assert.Error(t, err, "referencing a foreign order must be forbidden")

	// A non-member customer cannot post into the conversation.
	_, err = svc.SendMessage(context.Background(), conv.ConversationUuid, strangerUUID, "user", chatdto.SendMessageRequest{Content: "intrude"})
	assert.Error(t, err)

	// Admin reply lands with role admin.
	adminMsg, err := svc.SendMessage(context.Background(), conv.ConversationUuid, adminUUID, "admin", chatdto.SendMessageRequest{Content: "support here"})
	assert.NoError(t, err)
	assert.Equal(t, "admin", adminMsg.SenderRole)
}

func TestRecallMessage_Window(t *testing.T) {
	repo := newMockChatRepo()
	users := &mockUserRepo{users: map[uuid.UUID]chatservice.User{}}
	custUUID, customer := activeCustomer(10)
	users.users[custUUID] = customer
	svc, _, _ := newService(repo, users, &mockOrderReader{}, noTxManager{})

	conv, err := svc.GetOrCreateConversation(context.Background(), custUUID)
	assert.NoError(t, err)

	msg, err := svc.SendMessage(context.Background(), conv.ConversationUuid, custUUID, "user", chatdto.SendMessageRequest{Content: "oops"})
	assert.NoError(t, err)

	recalled, err := svc.RecallMessage(context.Background(), msg.UUID, custUUID)
	assert.NoError(t, err)
	assert.Equal(t, "recalled", recalled.ContentType)

	// Recall again is idempotent.
	recalled, err = svc.RecallMessage(context.Background(), msg.UUID, custUUID)
	assert.NoError(t, err)
	assert.Equal(t, "recalled", recalled.ContentType)

	// An old message outside the 60-minute window cannot be recalled.
	old := sqlc.ChatMessage{
		MessageID:      999,
		MessageUuid:    uuid.New(),
		ConversationID: conv.ConversationID,
		SenderID:       10,
		SenderRole:     "user",
		Content:        "ancient",
		ContentType:    "text",
		CreatedAt:      time.Now().Add(-2 * time.Hour),
	}
	repo.messages[old.MessageUuid] = old
	_, err = svc.RecallMessage(context.Background(), old.MessageUuid, custUUID)
	assert.Error(t, err)
}

func TestPresenceInResponses(t *testing.T) {
	repo := newMockChatRepo()
	users := &mockUserRepo{users: map[uuid.UUID]chatservice.User{}}
	custUUID, customer := activeCustomer(10)
	adminUUID, admin := activeAdmin(20)
	users.users[custUUID] = customer
	users.users[adminUUID] = admin

	svc, publisher, presence := newService(repo, users, &mockOrderReader{}, noTxManager{})
	presence.onlineUserIDs[10] = true
	presence.adminOnline = true

	conv, err := svc.GetOrCreateConversation(context.Background(), custUUID)
	assert.NoError(t, err)

	details, err := svc.GetConversationDetails(context.Background(), conv.ConversationUuid, adminUUID)
	assert.NoError(t, err)
	assert.True(t, details.IsOnline)
	assert.True(t, details.IsShopOnline)

	status, err := svc.GetShopStatus(context.Background())
	assert.NoError(t, err)
	assert.True(t, status.IsOnline)

	repo.conversations[conv.ConversationID] = sqlc.ChatConversation{
		ConversationID:   conv.ConversationID,
		ConversationUuid: conv.ConversationUuid,
		UserID:           10,
		Status:           "open",
		LastMessageAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UserEmail:        "u@test.dev",
		UserFullname:     "Customer",
	}
	_, _, _ = svc.ListConversations(context.Background(), adminUUID, chatdto.ListConversationsRequest{})
	_ = publisher
	assert.Equal(t, ws.PresenceKey, "ws:presence:conns", "shared presence key contract")
}

// A new conversation must store the denormalized user snapshot, otherwise the
// admin list renders a "?" avatar with no name.
func TestGetOrCreateConversation_StoresUserSnapshot(t *testing.T) {
	repo := newMockChatRepo()
	users := &mockUserRepo{users: map[uuid.UUID]chatservice.User{}, byID: map[int32]chatservice.User{}}
	svc, _, _ := newService(repo, users, &mockOrderReader{}, noTxManager{})

	custUUID, customer := activeCustomer(10)
	customer.UserFullname = "Nguyen Van A"
	customer.UserAvatar = "https://cdn/avatar.png"
	users.users[custUUID] = customer

	_, err := svc.GetOrCreateConversation(context.Background(), custUUID)
	assert.NoError(t, err)

	stored := repo.conversations[1]
	assert.Equal(t, "u@test.dev", stored.UserEmail)
	assert.Equal(t, "Nguyen Van A", stored.UserFullname)
	assert.Equal(t, "https://cdn/avatar.png", stored.UserAvatar)
}

// Conversations seeded from the monolith have empty user columns; the admin
// list must resolve them from identity and persist the result.
func TestListConversations_BackfillsLegacyRows(t *testing.T) {
	repo := newMockChatRepo()
	custUUID, customer := activeCustomer(10)
	customer.UserFullname = "Tran Thi B"
	customer.UserAvatar = "https://cdn/b.png"

	users := &mockUserRepo{
		users: map[uuid.UUID]chatservice.User{custUUID: customer},
		byID:  map[int32]chatservice.User{10: customer},
	}
	adminUUID, admin := activeAdmin(20)
	users.users[adminUUID] = admin

	conv := sqlc.ChatConversation{ConversationID: 1, ConversationUuid: uuid.New(), UserID: 10, Status: "open"}
	repo.conversations[1] = conv
	repo.listRows = []sqlc.ListConversationsRow{{
		ConversationID:   1,
		ConversationUuid: conv.ConversationUuid,
		UserID:           10,
		Status:           "open",
	}}

	svc, _, _ := newService(repo, users, &mockOrderReader{}, noTxManager{})
	rows, _, err := svc.ListConversations(context.Background(), adminUUID, chatdto.ListConversationsRequest{})
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Tran Thi B", rows[0].UserFullname)
	assert.Equal(t, "https://cdn/b.png", rows[0].UserAvatar)

	persisted := repo.conversations[1]
	assert.Equal(t, "Tran Thi B", persisted.UserFullname, "snapshot must be persisted to avoid re-fetching every list")
	assert.Equal(t, "u@test.dev", persisted.UserEmail)
}
