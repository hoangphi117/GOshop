include .env
export

MIGRATION_IDENTITY_DIR=services/identity/internal/migrations
IDENTITY_DB_NAME?=shopping_identity

CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}
IDENTITY_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${IDENTITY_DB_NAME}?sslmode=${DB_SSL_MODE}

PROD_COMPOSE=../docker-compose.prod.yml
NOAPP_COMPOSE=../docker-compose.noapp.yml
DEV_COMPOSE=../docker-compose.dev.yml

ENV_FILE=.env

importdb:
	docker exec -i postgres-db psql -U postgres -d master-golang < ./backupdb.sql

exportdb:
	docker exec -i postgres-db pg_dump -U postgres -d master-golang > ./backupdb.sql

seed:
	go run ./cmd/seed

seed-docker:
	docker exec -i postgres-db psql -U postgres -d master-golang < ../scripts/seed.sql

server:
	go run ./cmd/api

chat:
	go run ./cmd/chat

build:
	go build -o bin/goshop ./cmd/api

build-chat:
	go build -o bin/chat ./cmd/chat

media:
	go run ./cmd/media

build-media:
	go build -o bin/media ./cmd/media

run-binary:
	./bin/goshop

sqlc:
	sqlc generate

swagger:
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	npx --yes swagger2openapi ./docs/swagger.json -o ./docs/openapi.json

swagger-chat:
	swag init -g cmd/chat/main.go -o services/chat/docs --parseDependency --parseInternal
	npx --yes swagger2openapi ./services/chat/docs/swagger.json -o ./services/chat/docs/openapi.json

migrate-create:
	migrate create -ext sql -dir ${MIGRATION_IDENTITY_DIR} -seq ${NAME}

migrate-identity-create:
	migrate create -ext sql -dir ${MIGRATION_IDENTITY_DIR} -seq ${NAME}

migrate-up: migrate-identity-up


migrate-identity-up:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" up

# Rollback the last migration
migrate-down: migrate-identity-down


migrate-identity-down:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" down 1



migrate-identity-force:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" force ${VERSION}



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

# ---- Kafka operations ---------------------------------------------------
# The same targets live in the repo-root Makefile. They are repeated here
# because make only reads the Makefile in the current directory, and this is
# where service work happens. ROOT keeps the scripts reachable from backend/.

ROOT := $(abspath ..)

# Ad-hoc Grafana Cloud Loki query (needs GRAFANA_LOKI_READ_TOKEN in .env).
#   make logq Q='{container=goshop-order}' HOURS=6
# Make strips bare double quotes; a value written without them is quoted by
# the script, so Q='{container=goshop-order}' is enough.
logq:
	powershell -NoProfile -ExecutionPolicy Bypass -File "$(ROOT)/scripts/logq.ps1" '$(Q)' -Hours $(or $(HOURS),2) -Limit $(or $(LIMIT),10)

# Every DLQ topic must be empty in steady state.
check-dlq:
	powershell -NoProfile -ExecutionPolicy Bypass -File "$(ROOT)/scripts/check-dlq.ps1"

# Truncate DLQ topics in place, keeping the topics and their ACLs.
clear-dlq:
	powershell -NoProfile -ExecutionPolicy Bypass -File "$(ROOT)/scripts/clear-dlq.ps1"

# Healthz + consumer lag for the six Kafka consumer services.
kafka-health:
	powershell -NoProfile -ExecutionPolicy Bypass -File "$(ROOT)/scripts/check-kafka-health.ps1"

.PHONY: importdb exportdb server chat media migrate-create migrate-up migrate-down sqlc build build-chat build-media run-binary prod stop-prod logs-prod bash noapp stop-noapp dev stop-dev swagger swagger-chat seed seed-docker logq check-dlq clear-dlq kafka-health
