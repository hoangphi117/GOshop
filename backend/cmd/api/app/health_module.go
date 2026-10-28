package app

import (
	handler "goshop/cmd/api/handler"
	"goshop/cmd/api/routes"

	"goshop/pkg/discord"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/storage"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type HealthModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewHealthModule(
	dbPool handler.Pinger,
	redisClient *redis.Client,
	rabbitmqService rabbitmq.RabbitMQService,
	storageService storage.StorageService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
) *HealthModule {
	handler := handler.NewHealthHandler(dbPool, redisClient, rabbitmqService, storageService, discordNotifier, logger)
	healthRoutes := routes.NewHealthRoutes(handler)
	healthAdminRoutes := routes.NewHealthAdminRoutes(handler)

	return &HealthModule{
		routes:      []routes.Route{healthRoutes},
		adminRoutes: []routes.Route{healthAdminRoutes},
	}
}

func (m *HealthModule) Routes() []routes.Route {
	return m.routes
}

func (m *HealthModule) AdminRoutes() []routes.Route {
	return m.adminRoutes
}
