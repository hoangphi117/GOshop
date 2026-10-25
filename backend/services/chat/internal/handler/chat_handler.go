package handler

import (
	"net/http"

	"goshop/internal/utils"
	"goshop/internal/validation"
	"goshop/pkg/middleware"
	chatdto "goshop/services/chat/internal/dto"
	chatservice "goshop/services/chat/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ChatHandler struct {
	chatService chatservice.ChatService
}

func NewChatHandler(chatService chatservice.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

// -------------------------------------------------------------------------
// Storefront / Customer Endpoints
// -------------------------------------------------------------------------

// GetMyConversation godoc
// @Summary      Get or create customer conversation
// @Description  Retrieve the active conversation for the authenticated customer
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} utils.APIResponse{data=chatdto.ConversationResponse}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/conversation [get]
func (h *ChatHandler) GetMyConversation(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	conv, err := h.chatService.GetOrCreateConversation(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	details, err := h.chatService.GetConversationDetails(ctx.Request.Context(), conv.ConversationUuid, userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Conversation retrieved successfully", details)
}

// GetMyMessages godoc
// @Summary      Get customer messages
// @Description  Retrieve paginated messages for the customer's active conversation
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Param        page     query int false "Page number" default(1)
// @Param        limit    query int false "Items per page" default(20)
// @Param        before_id query int false "Cursor before ID"
// @Success      200 {object} utils.APIResponse{data=[]chatdto.MessageResponse}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/messages [get]
func (h *ChatHandler) GetMyMessages(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var req chatdto.ListMessagesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	conv, err := h.chatService.GetOrCreateConversation(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	messages, total, err := h.chatService.ListMessages(ctx.Request.Context(), conv.ConversationUuid, userUUID, req)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := max(req.Page, 1)
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}

	paginationResponse := utils.NewPaginationResponse(messages, page, limit, int32(total))
	utils.ResponseSuccess(ctx, http.StatusOK, "Messages retrieved successfully", paginationResponse)
}

// SendMessage godoc
// @Summary      Send chat message
// @Description  Send a message inside customer conversation
// @Tags         Chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body chatdto.SendMessageRequest true "Message content payload"
// @Success      201 {object} utils.APIResponse{data=chatdto.SendMessageResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/messages [post]
func (h *ChatHandler) SendMessage(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var req chatdto.SendMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	conv, err := h.chatService.GetOrCreateConversation(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	msg, err := h.chatService.SendMessage(ctx.Request.Context(), conv.ConversationUuid, userUUID, "user", req)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Message sent successfully", chatdto.SendMessageResponse{Message: msg})
}

// MarkAsRead godoc
// @Summary      Mark customer messages as read
// @Description  Mark unread messages in conversation as read
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} utils.APIResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/messages/read [patch]
func (h *ChatHandler) MarkAsRead(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	conv, err := h.chatService.GetOrCreateConversation(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if err := h.chatService.MarkRead(ctx.Request.Context(), conv.ConversationUuid, userUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Messages marked as read", nil)
}

// RecallMessage godoc
// @Summary      Recall message
// @Description  Recall a sent message within 60 minutes
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id  path string true "Message UUID"
// @Success      200 {object} utils.APIResponse{data=chatdto.MessageResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/messages/{id}/recall [patch]
func (h *ChatHandler) RecallMessage(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	msgUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid message UUID"))
		return
	}

	msg, err := h.chatService.RecallMessage(ctx.Request.Context(), msgUUID, userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Message recalled successfully", msg)
}

// GetUnreadCount godoc
// @Summary      Get customer unread message count
// @Description  Get the number of unread messages for the customer
// @Tags         Chat
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} utils.APIResponse{data=map[string]int64}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/unread-count [get]
func (h *ChatHandler) GetUnreadCount(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	count, err := h.chatService.GetTotalUnreadCount(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Unread count retrieved successfully", gin.H{"unread_count": count})
}

// SendTyping handles customer typing status update
func (h *ChatHandler) SendTyping(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var req chatdto.SendTypingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	conv, err := h.chatService.GetOrCreateConversation(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if err := h.chatService.SendTyping(ctx.Request.Context(), conv.ConversationUuid, userUUID, req.IsTyping); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Typing event sent", nil)
}

// GetShopStatus godoc
// @Summary      Get shop online status
// @Description  Check whether any shop admin is currently online
// @Tags         Chat
// @Produce      json
// @Success      200 {object} utils.APIResponse{data=chatdto.ShopStatusResponse}
// @Failure      500 {object} utils.ErrorResponse
// @Router       /chat/shop-status [get]
func (h *ChatHandler) GetShopStatus(ctx *gin.Context) {
	status, err := h.chatService.GetShopStatus(ctx.Request.Context())
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Shop status retrieved successfully", status)
}

// -------------------------------------------------------------------------
// Admin / Dashboard Endpoints
// -------------------------------------------------------------------------

// AdminListConversations godoc
// @Summary      List all chat conversations
// @Description  Admin list of customer conversations with filtering and pagination
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        status query string false "Filter by status: open, closed, pending"
// @Param        search query string false "Search keyword in user name or email"
// @Param        page   query int    false "Page number" default(1)
// @Param        limit  query int    false "Items per page" default(20)
// @Success      200 {object} utils.APIResponse{data=[]chatdto.ConversationResponse}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations [get]
func (h *ChatHandler) AdminListConversations(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var req chatdto.ListConversationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	conversations, total, err := h.chatService.ListConversations(ctx.Request.Context(), adminUUID, req)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := max(req.Page, 1)
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}

	paginationResponse := utils.NewPaginationResponse(conversations, page, limit, int32(total))
	utils.ResponseSuccess(ctx, http.StatusOK, "Conversations retrieved successfully", paginationResponse)
}

// AdminGetConversation godoc
// @Summary      Get conversation details
// @Description  Admin get details of a specific conversation
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id  path string true "Conversation UUID"
// @Success      200 {object} utils.APIResponse{data=chatdto.ConversationResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations/{id} [get]
func (h *ChatHandler) AdminGetConversation(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	details, err := h.chatService.GetConversationDetails(ctx.Request.Context(), convUUID, adminUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Conversation details retrieved successfully", details)
}

// AdminListMessages godoc
// @Summary      List messages in conversation
// @Description  Admin get paginated messages in a conversation
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string true "Conversation UUID"
// @Param        page     query int    false "Page number" default(1)
// @Param        limit    query int    false "Items per page" default(20)
// @Param        before_id query int   false "Cursor before ID"
// @Success      200 {object} utils.APIResponse{data=[]chatdto.MessageResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations/{id}/messages [get]
func (h *ChatHandler) AdminListMessages(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	var req chatdto.ListMessagesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	messages, total, err := h.chatService.ListMessages(ctx.Request.Context(), convUUID, adminUUID, req)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := max(req.Page, 1)
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}

	paginationResponse := utils.NewPaginationResponse(messages, page, limit, int32(total))
	utils.ResponseSuccess(ctx, http.StatusOK, "Messages retrieved successfully", paginationResponse)
}

// AdminSendMessage godoc
// @Summary      Admin send reply message
// @Description  Send an admin reply message to a conversation
// @Tags         Admin - Chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string true "Conversation UUID"
// @Param        body body chatdto.SendMessageRequest true "Message payload"
// @Success      201 {object} utils.APIResponse{data=chatdto.SendMessageResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations/{id}/messages [post]
func (h *ChatHandler) AdminSendMessage(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	var req chatdto.SendMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	msg, err := h.chatService.SendMessage(ctx.Request.Context(), convUUID, adminUUID, "admin", req)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Message sent successfully", chatdto.SendMessageResponse{Message: msg})
}

// AdminMarkAsRead godoc
// @Summary      Admin mark conversation as read
// @Description  Mark customer messages in a conversation as read by admin
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id  path string true "Conversation UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations/{id}/read [patch]
func (h *ChatHandler) AdminMarkAsRead(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	if err := h.chatService.MarkRead(ctx.Request.Context(), convUUID, adminUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Messages marked as read", nil)
}

// AdminCloseConversation godoc
// @Summary      Close conversation
// @Description  Mark conversation status as closed
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id  path string true "Conversation UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations/{id}/close [patch]
func (h *ChatHandler) AdminCloseConversation(ctx *gin.Context) {
	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	conv, err := h.chatService.CloseConversation(ctx.Request.Context(), convUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Conversation closed successfully", conv)
}

// AdminReopenConversation godoc
// @Summary      Reopen conversation
// @Description  Mark conversation status as active
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id  path string true "Conversation UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/conversations/{id}/reopen [patch]
func (h *ChatHandler) AdminReopenConversation(ctx *gin.Context) {
	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	conv, err := h.chatService.ReopenConversation(ctx.Request.Context(), convUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Conversation reopened successfully", conv)
}

// AdminRecallMessage godoc
// @Summary      Admin recall message
// @Description  Admin recall a message within 60 minutes
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        id  path string true "Message UUID"
// @Success      200 {object} utils.APIResponse{data=chatdto.MessageResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/messages/{id}/recall [patch]
func (h *ChatHandler) AdminRecallMessage(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	msgUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid message UUID"))
		return
	}

	msg, err := h.chatService.RecallMessage(ctx.Request.Context(), msgUUID, adminUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Message recalled successfully", msg)
}

// AdminGetUnreadCount godoc
// @Summary      Get admin unread message count
// @Description  Get the total count of unread messages across all conversations for admin
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} utils.APIResponse{data=map[string]int64}
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/unread-count [get]
func (h *ChatHandler) AdminGetUnreadCount(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	count, err := h.chatService.GetTotalUnreadCount(ctx.Request.Context(), adminUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Admin total unread count retrieved successfully", gin.H{"unread_count": count})
}

// AdminSendTyping handles admin typing status update for a conversation
func (h *ChatHandler) AdminSendTyping(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	convUUID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
		return
	}

	var req chatdto.SendTypingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := h.chatService.SendTyping(ctx.Request.Context(), convUUID, adminUUID, req.IsTyping); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Typing event sent", nil)
}

// AdminGetOrCreateUserConversation godoc
// @Summary      Get or create conversation for a customer
// @Description  Admin get existing conversation with a user or create a new one
// @Tags         Admin - Chat
// @Produce      json
// @Security     BearerAuth
// @Param        user_uuid path string true "User UUID"
// @Success      200 {object} utils.APIResponse{data=chatdto.ConversationResponse}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /admin/chat/users/{user_uuid}/conversation [post]
func (h *ChatHandler) AdminGetOrCreateUserConversation(ctx *gin.Context) {
	adminUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	userUUID, err := uuid.Parse(ctx.Param("user_uuid"))
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID"))
		return
	}

	conv, err := h.chatService.GetOrCreateConversation(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	details, err := h.chatService.GetConversationDetails(ctx.Request.Context(), conv.ConversationUuid, adminUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Conversation retrieved successfully", details)
}
