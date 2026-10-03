include .env
export

MIGRATION_DIR=internal/db/migrations

CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}

PROD_COMPOSE=../docker-compose.prod.yml
NOAPP_COMPOSE=../docker-compose.noapp.yml
DEV_COMPOSE=../docker-compose.dev.yml

ENV_FILE=.env

importdb:
	docker exec -i postgres-db psql -U postgres -d master-golang < ./backupdb.sql

exportdb:
	docker exec -i postgres-db pg_dump -U postgres -d master-golang > ./backupdb.sql

server:
	go run ./cmd/api

build:
	go build -o bin/goshop ./cmd/api

run-binary:
	./bin/goshop

sqlc:
	sqlc generate

swagger:
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	npx --yes swagger2openapi ./docs/swagger.json -o ./docs/openapi.json

migrate-create:
	migrate create -ext sql -dir ${MIGRATION_DIR} -seq ${NAME}

migrate-up:
	migrate -path ${MIGRATION_DIR} -database "${CONNECTION_STRING}" up

# Rollback the last migration
migrate-down:
	migrate -path ${MIGRATION_DIR} -database "${CONNECTION_STRING}" down 1

# Rollback the last N migrations
migrate-down-n:
	migrate -path ${MIGRATION_DIR} -database "${CONNECTION_STRING}" down ${N}

# Force the migration version to a specific version when the database is in a dirty state
migrate-force:
	migrate -path ${MIGRATION_DIR} -database "${CONNECTION_STRING}" force ${VERSION}

# Drop all tables in the database
migrate-drop:
	migrate -path ${MIGRATION_DIR} -database "${CONNECTION_STRING}" drop 

# Migrate to a specific version
migrate-goto:
	migrate -path ${MIGRATION_DIR} -database "${CONNECTION_STRING}" goto ${VERSION}

# Build container for noapp
noapp:
	docker-compose -f ${NOAPP_COMPOSE} down
	docker-compose -f ${NOAPP_COMPOSE} --env-file ${ENV_FILE} up -d --build

# Stop noapp containers
stop-noapp:
	docker-compose -f ${NOAPP_COMPOSE} down

# Build container for dev
dev:
	docker-compose -f ${DEV_COMPOSE} down
	docker-compose -f ${DEV_COMPOSE} --env-file ${ENV_FILE} up --build

# Stop dev containers
stop-dev:
	docker-compose -f ${DEV_COMPOSE} down

# Build container for production
prod:
	docker-compose -f ${PROD_COMPOSE} down
	docker-compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} up -d --build

# Stop production containers
stop-prod:
	docker-compose -f ${PROD_COMPOSE} down

# View production logs
logs-prod:
	docker-compose -f ${PROD_COMPOSE} logs -f --tail=100

# Enter bash shell in the API container
bash:
	docker exec -it goshop-api /bin/sh

.PHONY: importdb exportdb server migrate-create migrate-up migrate-down migrate-force migrate-drop migrate-goto migrate-down-n sqlc build run-binary prod stop-prod logs-prod bash noapp stop-noapp dev stop-dev swagger
