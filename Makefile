include ./backend/.env
export

MIGRATION_CORE_DIR=backend/internal/db/migrations_core
MIGRATION_IDENTITY_DIR=backend/internal/db/migrations_identity
MIGRATION_COUPON_DIR=backend/services/coupon/internal/migrations
MIGRATION_INVENTORY_DIR=backend/services/inventory/internal/migrations
MIGRATION_CATALOG_DIR=backend/services/catalog/internal/migrations
MIGRATION_CART_DIR=backend/services/cart/internal/migrations
MIGRATION_ORDER_DIR=backend/services/order/internal/migrations
MIGRATION_CHAT_DIR=backend/services/chat/internal/migrations
IDENTITY_DB_NAME?=shopping_identity
COUPON_DB_NAME?=shopping_coupon
INVENTORY_DB_NAME?=shopping_inventory
CATALOG_DB_NAME?=shopping_catalog
CART_DB_NAME?=shopping_cart
ORDER_DB_NAME?=shopping_order
CHAT_DB_NAME?=shopping_chat

CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}
IDENTITY_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${IDENTITY_DB_NAME}?sslmode=${DB_SSL_MODE}
COUPON_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${COUPON_DB_NAME}?sslmode=${DB_SSL_MODE}
INVENTORY_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${INVENTORY_DB_NAME}?sslmode=${DB_SSL_MODE}
CATALOG_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${CATALOG_DB_NAME}?sslmode=${DB_SSL_MODE}
CART_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${CART_DB_NAME}?sslmode=${DB_SSL_MODE}
ORDER_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${ORDER_DB_NAME}?sslmode=${DB_SSL_MODE}
CHAT_CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${CHAT_DB_NAME}?sslmode=${DB_SSL_MODE}

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

identity:
	cd backend && go run ./cmd/identity

build-identity:
	cd backend && go build -o bin/identity ./cmd/identity

notification:
	cd backend && go run ./cmd/notification

build-notification:
	cd backend && go build -o bin/notification ./cmd/notification

payment:
	cd backend && go run ./cmd/payment

build-payment:
	cd backend && go build -o bin/payment ./cmd/payment

coupon:
	cd backend && go run ./cmd/coupon

build-coupon:
	cd backend && go build -o bin/coupon ./cmd/coupon

inventory:
	cd backend && go run ./cmd/inventory

build-inventory:
	cd backend && go build -o bin/inventory ./cmd/inventory

catalog:
	cd backend && go run ./cmd/catalog

build-catalog:
	cd backend && go build -o bin/catalog ./cmd/catalog

cart:
	cd backend && go run ./cmd/cart

build-cart:
	cd backend && go build -o bin/cart ./cmd/cart

order:
	cd backend && go run ./cmd/order

build-order:
	cd backend && go build -o bin/order ./cmd/order

chat:
	cd backend && go run ./cmd/chat

build-chat:
	cd backend && go build -o bin/chat ./cmd/chat

media:
	cd backend && go run ./cmd/media

build-media:
	cd backend && go build -o bin/media ./cmd/media


run-binary:
	./backend/bin/goshop

sqlc:
	cd backend && sqlc generate

swagger:
	cd backend && swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	npx --yes swagger2openapi ./backend/docs/swagger.json -o ./backend/docs/openapi.json

migrate-create:
	migrate create -ext sql -dir ${MIGRATION_CORE_DIR} -seq ${NAME}

migrate-identity-create:
	migrate create -ext sql -dir ${MIGRATION_IDENTITY_DIR} -seq ${NAME}

migrate-up: migrate-core-up migrate-identity-up migrate-coupon-up migrate-inventory-up migrate-catalog-up migrate-cart-up migrate-order-up

migrate-core-up:
	migrate -path ${MIGRATION_CORE_DIR} -database "${CONNECTION_STRING}" up

migrate-identity-up:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" up

migrate-coupon-up:
	migrate -path ${MIGRATION_COUPON_DIR} -database "${COUPON_CONNECTION_STRING}" up

migrate-inventory-up:
	migrate -path ${MIGRATION_INVENTORY_DIR} -database "${INVENTORY_CONNECTION_STRING}" up

migrate-catalog-up:
	migrate -path ${MIGRATION_CATALOG_DIR} -database "${CATALOG_CONNECTION_STRING}" up

migrate-cart-up:
	migrate -path ${MIGRATION_CART_DIR} -database "${CART_CONNECTION_STRING}" up

migrate-order-up:
	migrate -path ${MIGRATION_ORDER_DIR} -database "${ORDER_CONNECTION_STRING}" up

migrate-chat-up:
	migrate -path ${MIGRATION_CHAT_DIR} -database "${CHAT_CONNECTION_STRING}" up

# Rollback the last migration
migrate-down: migrate-core-down migrate-identity-down migrate-coupon-down migrate-inventory-down migrate-catalog-down migrate-cart-down migrate-order-down

migrate-core-down:
	migrate -path ${MIGRATION_CORE_DIR} -database "${CONNECTION_STRING}" down 1

migrate-identity-down:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" down 1

migrate-coupon-down:
	migrate -path ${MIGRATION_COUPON_DIR} -database "${COUPON_CONNECTION_STRING}" down 1

migrate-inventory-down:
	migrate -path ${MIGRATION_INVENTORY_DIR} -database "${INVENTORY_CONNECTION_STRING}" down 1

migrate-catalog-down:
	migrate -path ${MIGRATION_CATALOG_DIR} -database "${CATALOG_CONNECTION_STRING}" down 1

migrate-cart-down:
	migrate -path ${MIGRATION_CART_DIR} -database "${CART_CONNECTION_STRING}" down 1

migrate-order-down:
	migrate -path ${MIGRATION_ORDER_DIR} -database "${ORDER_CONNECTION_STRING}" down 1

migrate-chat-down:
	migrate -path ${MIGRATION_CHAT_DIR} -database "${CHAT_CONNECTION_STRING}" down 1

# Rollback the last N migrations
migrate-down-n:
	migrate -path ${MIGRATION_CORE_DIR} -database "${CONNECTION_STRING}" down ${N}

# Force the migration version to a specific version when the database is in a dirty state
migrate-force:
	migrate -path ${MIGRATION_CORE_DIR} -database "${CONNECTION_STRING}" force ${VERSION}

migrate-identity-force:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" force ${VERSION}

# Drop all tables in the database
migrate-drop:
	migrate -path ${MIGRATION_CORE_DIR} -database "${CONNECTION_STRING}" drop 

# Migrate to a specific version
migrate-goto:
	migrate -path ${MIGRATION_CORE_DIR} -database "${CONNECTION_STRING}" goto ${VERSION}

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

# Run lightweight hybrid development (Infra in Docker, Go apps on Host)
dev-host:
	powershell -ExecutionPolicy Bypass -File ./scripts/dev.ps1

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

.PHONY: importdb exportdb server worker ws-gateway identity notification payment coupon inventory catalog order chat media build-identity build-notification build-payment build-coupon build-inventory build-catalog build-order build-chat build-media migrate-create migrate-up migrate-down migrate-force migrate-drop migrate-goto migrate-down-n sqlc build build-worker build-ws-gateway run-binary prod stop-prod logs-prod bash noapp stop-noapp dev stop-dev dev-host swagger loadtest stresstest spiketest seed seed-docker
