include ./backend/.env
export

MIGRATION_IDENTITY_DIR=backend/gomod/services/identity/internal/migrations
MIGRATION_COUPON_DIR=backend/gomod/services/coupon/internal/migrations
MIGRATION_INVENTORY_DIR=backend/gomod/services/inventory/internal/migrations
MIGRATION_CATALOG_DIR=backend/gomod/services/catalog/internal/migrations
MIGRATION_CART_DIR=backend/gomod/services/cart/internal/migrations
MIGRATION_ORDER_DIR=backend/gomod/services/order/internal/migrations
MIGRATION_CHAT_DIR=backend/gomod/services/chat/internal/migrations
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

# Switch environment: copy the variant over the active env files.
# The real .env.dev/.env.prod are git-ignored (secrets); if they are missing
# on a fresh clone, copy the committed .env.prod.example files and fill them.
# NEXT_PUBLIC_* are baked at storefront build time â€” rebuild after switching.
env-dev:
	powershell -NoProfile -Command "if (!(Test-Path backend/.env.dev) -or !(Test-Path frontend/apps/storefront/.env.dev)) { Write-Host 'Missing dev variant: cp backend/.env.dev.example backend/.env.dev + cp frontend/apps/storefront/.env.dev.example frontend/apps/storefront/.env.dev (secrets: see backend/.env.example)' -ForegroundColor Red; exit 1 }; Copy-Item backend/.env.dev backend/.env; Copy-Item frontend/apps/storefront/.env.dev frontend/apps/storefront/.env; Copy-Item frontend/apps/storefront/.env.dev frontend/apps/storefront/.env.production; Write-Host 'dev env active'"
env-prod:
	powershell -NoProfile -Command "if (!(Test-Path backend/.env.prod) -or !(Test-Path frontend/apps/storefront/.env.prod)) { Write-Host 'Missing prod variant: cp backend/.env.prod.example backend/.env.prod + cp frontend/apps/storefront/.env.prod.example frontend/apps/storefront/.env.prod, then fill every CHANGE_ME' -ForegroundColor Red; exit 1 }; Copy-Item backend/.env.prod backend/.env; Copy-Item frontend/apps/storefront/.env.prod frontend/apps/storefront/.env; Copy-Item frontend/apps/storefront/.env.prod frontend/apps/storefront/.env.production; Write-Host 'prod env active - set APP_ORIGIN in backend/.env.prod first; rebuild storefront + payment + api1 + api2'"

importdb:
	docker exec -i goshop-postgres psql -U ${DB_USER} -d ${DB_NAME} < ./backupdb.sql

exportdb:
	docker exec -i goshop-postgres pg_dump -U ${DB_USER} -d ${DB_NAME} > ./backupdb.sql

seed:
	cd backend/gomod/cmd && go run ./seed

seed-docker:
	docker exec -i goshop-postgres psql -U ${DB_USER} -d ${DB_NAME} < ./scripts/seed.sql

server:
	cd backend/gomod/cmd && go run ./api

worker:
	cd backend/gomod/cmd && go run ./worker

ws-gateway:
	cd backend/gomod/cmd && go run ./ws-gateway

build:
	cd backend/gomod/cmd && go build -o bin/goshop ./api

build-worker:
	cd backend/gomod/cmd && go build -o bin/worker ./worker

build-ws-gateway:
	cd backend/gomod/cmd && go build -o bin/ws-gateway ./ws-gateway

identity:
	cd backend/gomod/cmd && go run ./identity

build-identity:
	cd backend/gomod/cmd && go build -o bin/identity ./identity

notification:
	cd backend/gomod/cmd && go run ./notification

build-notification:
	cd backend/gomod/cmd && go build -o bin/notification ./notification

payment:
	cd backend/gomod/cmd && go run ./payment

build-payment:
	cd backend/gomod/cmd && go build -o bin/payment ./payment

coupon:
	cd backend/gomod/cmd && go run ./coupon

build-coupon:
	cd backend/gomod/cmd && go build -o bin/coupon ./coupon

inventory:
	cd backend/gomod/cmd && go run ./inventory

build-inventory:
	cd backend/gomod/cmd && go build -o bin/inventory ./inventory

catalog:
	cd backend/gomod/cmd && go run ./catalog

build-catalog:
	cd backend/gomod/cmd && go build -o bin/catalog ./catalog

cart:
	cd backend/gomod/cmd && go run ./cart

build-cart:
	cd backend/gomod/cmd && go build -o bin/cart ./cart

order:
	cd backend/gomod/cmd && go run ./order

build-order:
	cd backend/gomod/cmd && go build -o bin/order ./order

chat:
	cd backend/gomod/cmd && go run ./chat

build-chat:
	cd backend/gomod/cmd && go build -o bin/chat ./chat

media:
	cd backend/gomod/cmd && go run ./media

build-media:
	cd backend/gomod/cmd && go build -o bin/media ./media


run-binary:
	./backend/bin/goshop

sqlc:
	cd backend && sqlc generate

swagger:
	cd backend && swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	npx --yes swagger2openapi ./backend/docs/swagger.json -o ./backend/docs/openapi.json

migrate-create:
	migrate create -ext sql -dir ${MIGRATION_IDENTITY_DIR} -seq ${NAME}

migrate-identity-create:
	migrate create -ext sql -dir ${MIGRATION_IDENTITY_DIR} -seq ${NAME}

migrate-up: migrate-identity-up migrate-coupon-up migrate-inventory-up migrate-catalog-up migrate-cart-up migrate-order-up


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
migrate-down: migrate-identity-down migrate-coupon-down migrate-inventory-down migrate-catalog-down migrate-cart-down migrate-order-down


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



migrate-identity-force:
	migrate -path ${MIGRATION_IDENTITY_DIR} -database "${IDENTITY_CONNECTION_STRING}" force ${VERSION}



# Build container for noapp
noapp:
	docker compose -f ${NOAPP_COMPOSE} down
	docker compose -f ${NOAPP_COMPOSE} --env-file ${ENV_FILE} up -d --build

# Stop noapp containers
stop-noapp:
	docker compose -f ${NOAPP_COMPOSE} down

# Build container for dev
dev:
	docker compose -f ${DEV_COMPOSE} down
	docker compose -f ${DEV_COMPOSE} --env-file ${ENV_FILE} up --build

# Stop dev containers AND the host processes dev-host started. Without the
# second line a stopped stack leaves api.exe/analytics.exe etc. holding their
# ports, so the next dev-host dies with "bind: address already in use".
stop-dev:
	docker compose -f ${DEV_COMPOSE} down
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/stop-dev.ps1

# Run lightweight hybrid development (Infra in Docker, Go apps on Host)
dev-host:
	powershell -ExecutionPolicy Bypass -File ./scripts/dev.ps1

# Build container for production
prod:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-prod.ps1
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} build storefront
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} up -d
	powershell -NoProfile -Command "Start-Sleep -Seconds 30"
	$(MAKE) reload-nginx
	@echo ">>> done; now run: make smoke-prod"

# Admin SPA only (after a dashboard code change).
# Static build on the host: 1s via pnpm; nginx bind-mounts dist/ directly.
build-dashboard:
	cd frontend && pnpm --filter dashboard build

# Fast path: images already built, just (re)start the stack.
prod-fast:
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} up -d
	powershell -NoProfile -Command "Start-Sleep -Seconds 30"
	$(MAKE) reload-nginx
	@echo ">>> done; now run: make smoke-prod"

# Stop production containers
stop-prod:
	docker compose -f ${PROD_COMPOSE} down

# Post-deploy smoke check (8 HTTP probes + container states).
smoke-prod:
# On Windows the `bash` on PATH is the broken WSL relay, so route through
# Git Bash explicitly. On Linux dev boxes edit this to plain `bash`.
	powershell -NoProfile -Command "& 'C:\Program Files\Git\bin\bash.exe' scripts/smoke-prod.sh"

# Reset nginx upstream failure state after backend recovery.
reload-nginx:
	docker exec goshop-nginx nginx -s reload

# View production logs
logs-prod:
	docker compose -f ${PROD_COMPOSE} logs -f --tail=100

# Enter bash shell in the API container
bash:
	docker exec -it goshop-api /bin/sh

# Performance & Load Testing with Grafana k6 (Docker)
loadtest:
	docker run --rm -i --network=shopping_goshop-network -v "./scripts/loadtest:/loadtest" -e BASE_URL=http://goshop-nginx -e API_KEY=${API_KEY} grafana/k6 run /loadtest/load_test.js

stresstest:
	docker run --rm -i --network=shopping_goshop-network -v "./scripts/loadtest:/loadtest" -e BASE_URL=http://goshop-nginx -e API_KEY=${API_KEY} grafana/k6 run /loadtest/stress_test.js

spiketest:
	docker run --rm -i --network=shopping_goshop-network -v "./scripts/loadtest:/loadtest" -e BASE_URL=http://goshop-nginx -e API_KEY=${API_KEY} grafana/k6 run /loadtest/spike_test.js

.PHONY: importdb exportdb server worker ws-gateway identity notification payment coupon inventory catalog order chat media build-identity build-notification build-payment build-coupon build-inventory build-catalog build-order build-chat build-media migrate-create migrate-up migrate-down sqlc build build-worker build-ws-gateway run-binary prod stop-prod logs-prod bash noapp stop-noapp dev stop-dev dev-host swagger loadtest stresstest spiketest seed seed-docker env-dev env-prod

