package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestChatService_GetOrCreateConversation(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	userID := int32(10)

	mockActiveUser := sqlc.User{
		UserID:     userID,
		UserUuid:   userUUID,
		UserStatus: 1, // Active
	}

	existingConv := sqlc.ChatConversation{
		ConversationID: 1,
		UserID:         userID,
		Status:         "open",
	}

	t.Run("Success - Returns existing open conversation", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockActiveUser, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUserIDFunc: func(ctx context.Context, uID int32) (sqlc.ChatConversation, error) {
				return existingConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.GetOrCreateConversation(ctx, userUUID)

		assert.NoError(t, err)
		assert.Equal(t, existingConv.ConversationID, res.ConversationID)
	})

	t.Run("Success - Creates new conversation when none open", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockActiveUser, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUserIDFunc: func(ctx context.Context, uID int32) (sqlc.ChatConversation, error) {
				return sqlc.ChatConversation{}, pgx.ErrNoRows
			},
			CreateConversationFunc: func(ctx context.Context, uID int32) (sqlc.ChatConversation, error) {
				return existingConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.GetOrCreateConversation(ctx, userUUID)

		assert.NoError(t, err)
		assert.Equal(t, existingConv.ConversationID, res.ConversationID)
	})

	t.Run("Fail - User not found", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}
		mockChatRepo := &testutils.MockChatRepository{}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		_, err := svc.GetOrCreateConversation(ctx, userUUID)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeNotFound, appErr.Code)
	})

	t.Run("Fail - Inactive user cannot create conversation", func(t *testing.T) {
		mockInactiveUser := sqlc.User{UserID: userID, UserUuid: userUUID, UserStatus: 2}
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockInactiveUser, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		_, err := svc.GetOrCreateConversation(ctx, userUUID)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})
}

func TestChatService_SendMessage(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	adminUUID := uuid.New()
	strangerUUID := uuid.New()
	convUUID := uuid.New()

	mockCustomer := sqlc.User{UserID: 10, UserUuid: userUUID, UserLevel: 3, UserStatus: 1}
	mockAdmin := sqlc.User{UserID: 99, UserUuid: adminUUID, UserLevel: 1, UserStatus: 1}
	mockStranger := sqlc.User{UserID: 20, UserUuid: strangerUUID, UserLevel: 3, UserStatus: 1}

	mockOpenConv := sqlc.ChatConversation{
		ConversationID:   1,
		ConversationUuid: convUUID,
		UserID:           10,
		Status:           "open",
	}

	mockClosedConv := sqlc.ChatConversation{
		ConversationID:   2,
		ConversationUuid: convUUID,
		UserID:           10,
		Status:           "closed",
	}

	req := v1dto.SendMessageRequest{
		Content:     "Hello, I need assistance!",
		ContentType: "text",
	}

	t.Run("Success - Customer sends message with TxManager", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
			CreateMessageFunc: func(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
				return sqlc.ChatMessage{
					MessageUuid: uuid.New(),
					SenderID:    params.SenderID,
					SenderRole:  params.SenderRole,
					Content:     params.Content,
					ContentType: params.ContentType,
				}, nil
			},
			UpdateLastMessageAtFunc: func(ctx context.Context, id int32) error {
				return nil
			},
		}

		txMgr := testutils.NewNoopTxManager()
		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, txMgr, nil)
		res, err := svc.SendMessage(ctx, convUUID, userUUID, "user", req)

		assert.NoError(t, err)
		assert.Equal(t, "user", res.SenderRole)
		assert.Equal(t, req.Content, res.Content)
	})

	t.Run("Success - Admin sends message", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockAdmin, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
			CreateMessageFunc: func(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
				return sqlc.ChatMessage{
					MessageUuid: uuid.New(),
					SenderID:    params.SenderID,
					SenderRole:  params.SenderRole,
					Content:     params.Content,
					ContentType: params.ContentType,
				}, nil
			},
			UpdateLastMessageAtFunc: func(ctx context.Context, id int32) error {
				return nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.SendMessage(ctx, convUUID, adminUUID, "admin", req)

		assert.NoError(t, err)
		assert.Equal(t, "admin", res.SenderRole)
	})

	t.Run("Forbidden - Stranger sends message to another user's conversation", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockStranger, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		_, err := svc.SendMessage(ctx, convUUID, strangerUUID, "user", req)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})

	t.Run("Success - Sending message to closed conversation auto-reopens it", func(t *testing.T) {
		statusUpdated := false
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockClosedConv, nil
			},
			UpdateConversationStatusFunc: func(ctx context.Context, conversationID int32, status string) (sqlc.ChatConversation, error) {
				if status == "open" {
					statusUpdated = true
				}
				return mockOpenConv, nil
			},
			CreateMessageFunc: func(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
				return sqlc.ChatMessage{
					MessageUuid: uuid.New(),
					SenderID:    params.SenderID,
					SenderRole:  params.SenderRole,
					Content:     params.Content,
					ContentType: params.ContentType,
				}, nil
			},
			UpdateLastMessageAtFunc: func(ctx context.Context, conversationID int32) error {
				return nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		resp, err := svc.SendMessage(ctx, convUUID, userUUID, "user", req)

		assert.NoError(t, err)
		assert.Equal(t, req.Content, resp.Content)
		assert.True(t, statusUpdated)
	})

	t.Run("Success - Customer sends image attachment", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		imgURL := "https://cdn.example.com/uploads/chat/photo.jpg"
		imgName := "photo.jpg"
		imgSize := int64(102400)
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
			CreateMessageFunc: func(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
				assert.Equal(t, "image", params.ContentType)
				assert.Equal(t, &imgURL, params.AttachmentUrl)
				assert.Equal(t, &imgName, params.AttachmentName)
				assert.Equal(t, &imgSize, params.AttachmentSize)
				return sqlc.ChatMessage{
					MessageUuid:    uuid.New(),
					SenderID:       params.SenderID,
					SenderRole:     params.SenderRole,
					Content:        params.Content,
					ContentType:    params.ContentType,
					AttachmentUrl:  params.AttachmentUrl,
					AttachmentName: params.AttachmentName,
					AttachmentSize: params.AttachmentSize,
				}, nil
			},
			UpdateLastMessageAtFunc: func(ctx context.Context, id int32) error {
				return nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		attachReq := v1dto.SendMessageRequest{
			AttachmentURL:  imgURL,
			AttachmentName: imgName,
			AttachmentSize: imgSize,
		}
		res, err := svc.SendMessage(ctx, convUUID, userUUID, "user", attachReq)

		assert.NoError(t, err)
		assert.Equal(t, "image", res.ContentType)
		assert.Equal(t, imgURL, res.AttachmentURL)
		assert.Equal(t, imgName, res.AttachmentName)
		assert.Equal(t, imgSize, res.AttachmentSize)
	})

	t.Run("Success - Customer sends video attachment", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		videoURL := "https://cdn.example.com/uploads/chat/demo.mp4"
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
			CreateMessageFunc: func(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
				assert.Equal(t, "video", params.ContentType)
				return sqlc.ChatMessage{
					MessageUuid:   uuid.New(),
					SenderID:      params.SenderID,
					SenderRole:    params.SenderRole,
					Content:       params.Content,
					ContentType:   params.ContentType,
					AttachmentUrl: params.AttachmentUrl,
				}, nil
			},
			UpdateLastMessageAtFunc: func(ctx context.Context, id int32) error {
				return nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		attachReq := v1dto.SendMessageRequest{
			AttachmentURL: videoURL,
		}
		res, err := svc.SendMessage(ctx, convUUID, userUUID, "user", attachReq)

		assert.NoError(t, err)
		assert.Equal(t, "video", res.ContentType)
		assert.Equal(t, videoURL, res.AttachmentURL)
	})

	t.Run("Success - Customer references valid own order (order_ref)", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		orderUUID := uuid.New()
		mockOrderReader := &testutils.MockOrderReader{
			GetOrderSummaryFunc: func(ctx context.Context, u uuid.UUID) (*v1service.OrderSummary, error) {
				assert.Equal(t, orderUUID, u)
				return &v1service.OrderSummary{
					OrderID:   123,
					OrderUUID: orderUUID,
					UserID:    mockCustomer.UserID,
				}, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
			CreateMessageFunc: func(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error) {
				assert.Equal(t, "order_ref", params.ContentType)
				assert.Equal(t, orderUUID.String(), params.Content)
				return sqlc.ChatMessage{
					MessageUuid: uuid.New(),
					SenderID:    params.SenderID,
					SenderRole:  params.SenderRole,
					Content:     params.Content,
					ContentType: params.ContentType,
				}, nil
			},
			UpdateLastMessageAtFunc: func(ctx context.Context, id int32) error {
				return nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, mockOrderReader, nil, nil)
		orderReq := v1dto.SendMessageRequest{
			ContentType: "order_ref",
			OrderUUID:   orderUUID.String(),
		}
		res, err := svc.SendMessage(ctx, convUUID, userUUID, "user", orderReq)

		assert.NoError(t, err)
		assert.Equal(t, "order_ref", res.ContentType)
		assert.Equal(t, orderUUID.String(), res.Content)
	})

	t.Run("Forbidden - Customer references another user's order", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		orderUUID := uuid.New()
		mockOrderReader := &testutils.MockOrderReader{
			GetOrderSummaryFunc: func(ctx context.Context, u uuid.UUID) (*v1service.OrderSummary, error) {
				return &v1service.OrderSummary{
					OrderID:   123,
					OrderUUID: orderUUID,
					UserID:    9999, // different user
				}, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, mockOrderReader, nil, nil)
		orderReq := v1dto.SendMessageRequest{
			ContentType: "order_ref",
			OrderUUID:   orderUUID.String(),
		}
		_, err := svc.SendMessage(ctx, convUUID, userUUID, "user", orderReq)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})

	t.Run("NotFound - Customer references non-existent order", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		mockOrderReader := &testutils.MockOrderReader{
			GetOrderSummaryFunc: func(ctx context.Context, u uuid.UUID) (*v1service.OrderSummary, error) {
				return nil, pgx.ErrNoRows
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, mockOrderReader, nil, nil)
		orderReq := v1dto.SendMessageRequest{
			ContentType: "order_ref",
			OrderUUID:   uuid.New().String(),
		}
		_, err := svc.SendMessage(ctx, convUUID, userUUID, "user", orderReq)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeNotFound, appErr.Code)
	})

	t.Run("BadRequest - Empty content and empty attachment", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return mockCustomer, nil
			},
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) {
				return mockOpenConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		emptyReq := v1dto.SendMessageRequest{
			Content:     "",
			ContentType: "text",
		}
		_, err := svc.SendMessage(ctx, convUUID, userUUID, "user", emptyReq)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeBadRequest, appErr.Code)
	})
}

func TestChatService_ListMessages(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	adminUUID := uuid.New()
	convUUID := uuid.New()
	strangerUUID := uuid.New()

	mockUser := sqlc.User{UserID: 1, UserUuid: userUUID, UserLevel: 3, UserStatus: 1}
	mockAdmin := sqlc.User{UserID: 99, UserUuid: adminUUID, UserLevel: 1, UserStatus: 1}
	mockStranger := sqlc.User{UserID: 2, UserUuid: strangerUUID, UserLevel: 3, UserStatus: 1}
	mockConv := sqlc.ChatConversation{ConversationID: 10, ConversationUuid: convUUID, UserID: 1}

	t.Run("Success - Owner lists messages with pagination", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockUser, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
			ListMessagesFunc: func(ctx context.Context, params sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error) {
				return []sqlc.ChatMessage{
					{MessageID: 100, Content: "Hello", SenderID: 1, SenderRole: "user"},
				}, nil
			},
			CountMessagesByConversationIDFunc: func(ctx context.Context, id int32) (int64, error) { return 1, nil },
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, total, err := svc.ListMessages(ctx, convUUID, userUUID, v1dto.ListMessagesRequest{Limit: 20})

		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, res, 1)
		assert.Equal(t, "Hello", res[0].Content)
	})

	t.Run("Success - Cursor-based pagination with BeforeID", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockAdmin, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
			ListMessagesCursorFunc: func(ctx context.Context, params sqlc.ListMessagesCursorParams) ([]sqlc.ChatMessage, error) {
				assert.Equal(t, int32(50), params.BeforeID)
				return []sqlc.ChatMessage{
					{MessageID: 49, Content: "Older message", SenderID: 1, SenderRole: "user"},
				}, nil
			},
			CountMessagesByConversationIDFunc: func(ctx context.Context, id int32) (int64, error) { return 10, nil },
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, total, err := svc.ListMessages(ctx, convUUID, adminUUID, v1dto.ListMessagesRequest{BeforeID: 50, Limit: 10})

		assert.NoError(t, err)
		assert.Equal(t, int64(10), total)
		assert.Len(t, res, 1)
		assert.Equal(t, "Older message", res[0].Content)
	})

	t.Run("Forbidden - Stranger cannot list messages", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockStranger, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		_, _, err := svc.ListMessages(ctx, convUUID, strangerUUID, v1dto.ListMessagesRequest{})

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})

	t.Run("Success - List messages maps attachments properly", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockUser, nil },
		}
		imgURL := "https://cdn.example.com/uploads/chat/image.png"
		imgName := "image.png"
		imgSize := int64(2048)
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
			ListMessagesFunc: func(ctx context.Context, params sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error) {
				return []sqlc.ChatMessage{
					{
						MessageID:      101,
						Content:        "Photo sent",
						ContentType:    "image",
						AttachmentUrl:  &imgURL,
						AttachmentName: &imgName,
						AttachmentSize: &imgSize,
						SenderID:       1,
						SenderRole:     "user",
					},
					{
						MessageID:   102,
						Content:     "Text message",
						ContentType: "text",
						SenderID:    1,
						SenderRole:  "user",
					},
				}, nil
			},
			CountMessagesByConversationIDFunc: func(ctx context.Context, id int32) (int64, error) { return 2, nil },
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, total, err := svc.ListMessages(ctx, convUUID, userUUID, v1dto.ListMessagesRequest{})

		assert.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, res, 2)
		assert.Equal(t, imgURL, res[0].AttachmentURL)
		assert.Equal(t, imgName, res[0].AttachmentName)
		assert.Equal(t, imgSize, res[0].AttachmentSize)
		assert.Equal(t, "", res[1].AttachmentURL)
	})
}

func TestChatService_MarkRead(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	strangerUUID := uuid.New()
	convUUID := uuid.New()

	mockUser := sqlc.User{UserID: 1, UserUuid: userUUID, UserLevel: 3, UserStatus: 1}
	mockStranger := sqlc.User{UserID: 2, UserUuid: strangerUUID, UserLevel: 3, UserStatus: 1}
	mockConv := sqlc.ChatConversation{ConversationID: 10, ConversationUuid: convUUID, UserID: 1}

	t.Run("Success - Marks other participant's messages as read", func(t *testing.T) {
		markedReadCalled := false
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockUser, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
			MarkMessagesReadFunc: func(ctx context.Context, convID, readerID int32) error {
				markedReadCalled = true
				assert.Equal(t, int32(10), convID)
				assert.Equal(t, int32(1), readerID)
				return nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		err := svc.MarkRead(ctx, convUUID, userUUID)

		assert.NoError(t, err)
		assert.True(t, markedReadCalled)
	})

	t.Run("Forbidden - Stranger cannot mark read", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockStranger, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		err := svc.MarkRead(ctx, convUUID, strangerUUID)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})
}

func TestChatService_ListConversations(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	customerUUID := uuid.New()

	mockAdmin := sqlc.User{UserID: 99, UserUuid: adminUUID, UserLevel: 1, UserStatus: 1}
	mockCustomer := sqlc.User{UserID: 1, UserUuid: customerUUID, UserLevel: 3, UserStatus: 1}

	t.Run("Success - Admin lists all conversations with unread counts", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockAdmin, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			ListConversationsFunc: func(ctx context.Context, params sqlc.ListConversationsParams) ([]sqlc.ListConversationsRow, error) {
				return []sqlc.ListConversationsRow{
					{
						ConversationID:     1,
						ConversationUuid:   uuid.New(),
						Status:             "open",
						UserFullname:       "John Doe",
						UserEmail:          "john@example.com",
						UnreadCount:        3,
						LastMessageContent: "I need help with order #123",
					},
				}, nil
			},
			CountConversationsFunc: func(ctx context.Context, params sqlc.CountConversationsParams) (int64, error) {
				return 1, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, total, err := svc.ListConversations(ctx, adminUUID, v1dto.ListConversationsRequest{Page: 1, Limit: 20})

		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, res, 1)
		assert.Equal(t, "John Doe", res[0].UserFullname)
		assert.Equal(t, int64(3), res[0].UnreadCount)
	})

	t.Run("Forbidden - Customer cannot access admin conversation list", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockCustomer, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		_, _, err := svc.ListConversations(ctx, customerUUID, v1dto.ListConversationsRequest{})

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})
}

func TestChatService_GetConversationDetails(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	adminUUID := uuid.New()
	convUUID := uuid.New()
	strangerUUID := uuid.New()

	mockUser := sqlc.User{
		UserID:       10,
		UserUuid:     userUUID,
		UserLevel:    3,
		UserStatus:   1,
		UserFullname: "Alice",
		UserEmail:    "alice@example.com",
		UserAvatar:   "https://example.com/alice.jpg",
	}
	mockAdmin := sqlc.User{
		UserID:       99,
		UserUuid:     adminUUID,
		UserLevel:    1,
		UserStatus:   1,
		UserFullname: "Admin Bob",
		UserEmail:    "admin@example.com",
	}
	mockStranger := sqlc.User{UserID: 20, UserUuid: strangerUUID, UserLevel: 3, UserStatus: 1}
	mockConv := sqlc.ChatConversation{
		ConversationID:   1,
		ConversationUuid: convUUID,
		UserID:           10,
		Status:           "open",
		LastMessageAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	t.Run("Success - Customer views own conversation details", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockUser, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
			CountUnreadMessagesFunc: func(ctx context.Context, convID, readerID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.GetConversationDetails(ctx, convUUID, userUUID)

		assert.NoError(t, err)
		assert.Equal(t, mockConv.ConversationID, res.ConversationID)
		assert.Equal(t, "Alice", res.UserFullname)
		assert.Equal(t, "alice@example.com", res.UserEmail)
		assert.Equal(t, "https://example.com/alice.jpg", res.UserAvatar)
	})

	t.Run("Success - Admin views conversation details", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockAdmin, nil },
			GetByIDFunc:   func(ctx context.Context, id int32) (sqlc.User, error) { return mockUser, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
			CountUnreadMessagesFunc: func(ctx context.Context, convID, readerID int32) (int64, error) {
				return 2, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.GetConversationDetails(ctx, convUUID, adminUUID)

		assert.NoError(t, err)
		assert.Equal(t, mockConv.ConversationID, res.ConversationID)
		assert.Equal(t, int64(2), res.UnreadCount)
		assert.Equal(t, "Alice", res.UserFullname)
		assert.Equal(t, "alice@example.com", res.UserEmail)
		assert.Equal(t, "https://example.com/alice.jpg", res.UserAvatar)
	})

	t.Run("Forbidden - Stranger cannot view conversation details", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockStranger, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockConv, nil },
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		_, err := svc.GetConversationDetails(ctx, convUUID, strangerUUID)

		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeForbidden, appErr.Code)
	})
}

func TestChatService_CloseConversation(t *testing.T) {
	ctx := context.Background()
	convUUID := uuid.New()

	mockOpenConv := sqlc.ChatConversation{ConversationID: 1, ConversationUuid: convUUID, UserID: 10, Status: "open"}
	mockClosedConv := sqlc.ChatConversation{ConversationID: 1, ConversationUuid: convUUID, UserID: 10, Status: "closed"}

	t.Run("Success - Closes open conversation", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockOpenConv, nil },
			UpdateConversationStatusFunc: func(ctx context.Context, id int32, status string) (sqlc.ChatConversation, error) {
				assert.Equal(t, "closed", status)
				return sqlc.ChatConversation{ConversationID: id, Status: status}, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.CloseConversation(ctx, convUUID)

		assert.NoError(t, err)
		assert.Equal(t, "closed", res.Status)
	})

	t.Run("Success - Idempotent when conversation is already closed", func(t *testing.T) {
		updateStatusCalled := false
		mockUserRepo := &testutils.MockUserRepository{}
		mockChatRepo := &testutils.MockChatRepository{
			GetConversationByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.ChatConversation, error) { return mockClosedConv, nil },
			UpdateConversationStatusFunc: func(ctx context.Context, id int32, status string) (sqlc.ChatConversation, error) {
				updateStatusCalled = true
				return mockClosedConv, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		res, err := svc.CloseConversation(ctx, convUUID)

		assert.NoError(t, err)
		assert.Equal(t, "closed", res.Status)
		assert.False(t, updateStatusCalled)
	})
}

func TestChatService_GetTotalUnreadCount(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()
	userUUID := uuid.New()

	mockAdmin := sqlc.User{UserID: 99, UserUuid: adminUUID, UserLevel: 1, UserStatus: 1}
	mockUser := sqlc.User{UserID: 10, UserUuid: userUUID, UserLevel: 3, UserStatus: 1}

	t.Run("Success - Admin gets total unread from customers", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockAdmin, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetTotalUnreadMessagesForAdminFunc: func(ctx context.Context) (int64, error) {
				return 5, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		count, err := svc.GetTotalUnreadCount(ctx, adminUUID)

		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})

	t.Run("Success - User gets total unread from admin", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) { return mockUser, nil },
		}
		mockChatRepo := &testutils.MockChatRepository{
			GetTotalUnreadMessagesForUserFunc: func(ctx context.Context, uID int32) (int64, error) {
				return 2, nil
			},
		}

		svc := v1service.NewChatService(mockChatRepo, mockUserRepo, nil, nil, nil)
		count, err := svc.GetTotalUnreadCount(ctx, userUUID)

		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}
