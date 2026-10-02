package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type ChatRoutes struct {
	handler *v1handler.ChatHandler
}

func NewChatRoutes(handler *v1handler.ChatHandler) *ChatRoutes {
	return &ChatRoutes{handler: handler}
}

func (r *ChatRoutes) Register(rg *gin.RouterGroup) {
	chat := rg.Group("/chat")
	{
		chat.GET("/conversation", r.handler.GetMyConversation)
		chat.GET("/messages", r.handler.GetMyMessages)
		chat.POST("/messages", r.handler.SendMessage)
		chat.PATCH("/messages/read", r.handler.MarkAsRead)
		chat.GET("/unread-count", r.handler.GetUnreadCount)
		chat.POST("/typing", r.handler.SendTyping)
		chat.GET("/shop-status", r.handler.GetShopStatus)
	}
}
