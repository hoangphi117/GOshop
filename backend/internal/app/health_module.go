package app

import (
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/routes"
	v1routes "goshop/internal/routes/v1"
	"goshop/pkg/discord"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type HealthModule struct {
	routes      []routes.Route
	adminRoutes []routes.Route
}

func NewHealthModule(
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
	rabbitmqService rabbitmq.RabbitMQService,
	storageService storage.StorageService,
	discordNotifier discord.Notifier,
	logger *zerolog.Logger,
) *HealthModule {
	handler := v1handler.NewHealthHandler(dbPool, redisClient, rabbitmqService, storageService, discordNotifier, logger)
	healthRoutes := v1routes.NewHealthRoutes(handler)
	healthAdminRoutes := v1routes.NewHealthAdminRoutes(handler)

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
