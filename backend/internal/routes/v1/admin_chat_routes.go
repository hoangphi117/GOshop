package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminChatRoutes struct {
	handler *v1handler.ChatHandler
}

func NewAdminChatRoutes(handler *v1handler.ChatHandler) *AdminChatRoutes {
	return &AdminChatRoutes{handler: handler}
}

func (r *AdminChatRoutes) Register(rg *gin.RouterGroup) {
	adminChat := rg.Group("/chat")
	{
		adminChat.GET("/conversations", r.handler.AdminListConversations)
		adminChat.GET("/conversations/:id", r.handler.AdminGetConversation)
		adminChat.GET("/conversations/:id/messages", r.handler.AdminListMessages)
		adminChat.POST("/conversations/:id/messages", r.handler.AdminSendMessage)
		adminChat.PATCH("/conversations/:id/read", r.handler.AdminMarkAsRead)
		adminChat.PATCH("/conversations/:id/close", r.handler.AdminCloseConversation)
		adminChat.PATCH("/conversations/:id/reopen", r.handler.AdminReopenConversation)
		adminChat.PATCH("/messages/:id/recall", r.handler.AdminRecallMessage)
		adminChat.GET("/unread-count", r.handler.AdminGetUnreadCount)
		adminChat.POST("/conversations/:id/typing", r.handler.AdminSendTyping)
		adminChat.POST("/users/:user_uuid/conversation", r.handler.AdminGetOrCreateUserConversation)
	}
}
