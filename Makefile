include ./backend/.env
export

MIGRATION_DIR=backend/internal/db/migrations

CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}

PROD_COMPOSE=docker-compose.prod.yml
NOAPP_COMPOSE=docker-compose.noapp.yml
DEV_COMPOSE=docker-compose.dev.yml

ENV_FILE=./backend/.env

importdb:
	docker exec -i goshop-postgres psql -U ${DB_USER} -d ${DB_NAME} < ./backupdb.sql

exportdb:
	docker exec -i goshop-postgres pg_dump -U ${DB_USER} -d ${DB_NAME} > ./backupdb.sql

seed:
	cd backend && go run ./cmd/seed

seed-docker:
	docker exec -i goshop-postgres psql -U ${DB_USER} -d ${DB_NAME} < ./scripts/seed.sql

server:
	cd backend && go run ./cmd/api

worker:
	cd backend && go run ./cmd/worker

ws-gateway:
	cd backend && go run ./cmd/ws-gateway

build:
	cd backend && go build -o bin/goshop ./cmd/api

build-worker:
	cd backend && go build -o bin/worker ./cmd/worker

build-ws-gateway:
	cd backend && go build -o bin/ws-gateway ./cmd/ws-gateway

run-binary:
	./backend/bin/goshop

sqlc:
	cd backend && sqlc generate

swagger:
	cd backend && swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	npx --yes swagger2openapi ./backend/docs/swagger.json -o ./backend/docs/openapi.json

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

# Performance & Load Testing with Grafana k6 (Docker)
loadtest:
	docker run --rm -i --network=shopping_goshop-network -v "./loadtest:/loadtest" -e BASE_URL=http://goshop-nginx -e API_KEY=abcd-1234-1111 grafana/k6 run /loadtest/load_test.js

stresstest:
	docker run --rm -i --network=shopping_goshop-network -v "./loadtest:/loadtest" -e BASE_URL=http://goshop-nginx -e API_KEY=abcd-1234-1111 grafana/k6 run /loadtest/stress_test.js

spiketest:
	docker run --rm -i --network=shopping_goshop-network -v "./loadtest:/loadtest" -e BASE_URL=http://goshop-nginx -e API_KEY=abcd-1234-1111 grafana/k6 run /loadtest/spike_test.js

.PHONY: importdb exportdb server worker ws-gateway migrate-create migrate-up migrate-down migrate-force migrate-drop migrate-goto migrate-down-n sqlc build build-worker build-ws-gateway run-binary prod stop-prod logs-prod bash noapp stop-noapp dev stop-dev swagger loadtest stresstest spiketest seed seed-docker
