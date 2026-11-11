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
#
# .env.local outranks .env and .env.development, so it is rewritten too:
# leaving a stale copy made `next dev` keep pointing at the previous
# topology's port (browser hits a closed port after the switch).
env-dev:
	powershell -NoProfile -Command "if (!(Test-Path backend/.env.dev) -or !(Test-Path frontend/apps/storefront/.env.dev)) { Write-Host 'Missing dev variant: cp backend/.env.dev.example backend/.env.dev + cp frontend/apps/storefront/.env.dev.example frontend/apps/storefront/.env.dev (secrets: see backend/.env.example)' -ForegroundColor Red; exit 1 }; Copy-Item backend/.env.dev backend/.env; Copy-Item frontend/apps/storefront/.env.dev frontend/apps/storefront/.env; Copy-Item frontend/apps/storefront/.env.dev frontend/apps/storefront/.env.production; Remove-Item frontend/apps/storefront/.env.local -ErrorAction SilentlyContinue; Write-Host 'dev env active'"
env-prod:
	powershell -NoProfile -Command "if (!(Test-Path backend/.env.prod) -or !(Test-Path frontend/apps/storefront/.env.prod)) { Write-Host 'Missing prod variant: cp backend/.env.prod.example backend/.env.prod + cp frontend/apps/storefront/.env.prod.example frontend/apps/storefront/.env.prod, then fill every CHANGE_ME' -ForegroundColor Red; exit 1 }; Copy-Item backend/.env.prod backend/.env; Copy-Item frontend/apps/storefront/.env.prod frontend/apps/storefront/.env; Copy-Item frontend/apps/storefront/.env.prod frontend/apps/storefront/.env.production; Remove-Item frontend/apps/storefront/.env.local -ErrorAction SilentlyContinue; Write-Host 'prod env active - set APP_ORIGIN in backend/.env.prod first; rebuild storefront + payment + api1 + api2; then run make kafka-env (this copy has no Kafka SASL/TLS values)'"

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

# Build container for dev.
# Drops the dev-host override: with nginx on :80, a stale .env.local pointing
# at :8080 would shadow .env.development and the browser would hit a dead port.
dev:
	powershell -NoProfile -Command "Remove-Item frontend/apps/storefront/.env.local -ErrorAction SilentlyContinue"
	docker compose -f ${DEV_COMPOSE} down
	docker compose -f ${DEV_COMPOSE} --env-file ${ENV_FILE} up --build

# Stop dev containers AND the host processes dev-host started. Without the
# second line a stopped stack leaves api.exe/analytics.exe etc. holding their
# ports, so the next dev-host dies with "bind: address already in use".
stop-dev:
	docker compose -f ${DEV_COMPOSE} down
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/stop-dev.ps1

# Run lightweight hybrid development (Infra in Docker, Go apps on Host).
# No nginx in this topology, so the storefront must target :8080 instead of the
# edge on :80. .env.local outranks .env.development, which is why this copy is
# what keeps Google login and every client fetch pointing at a live port.
dev-host:
	powershell -NoProfile -Command "Copy-Item frontend/apps/storefront/.env.dev-host frontend/apps/storefront/.env.local -Force"
	powershell -ExecutionPolicy Bypass -File ./scripts/dev.ps1

# Verify every gateway-proxied route answers (fails on 502). Run after `make dev`
# or `make dev-host`: a service added to prod compose but not to dev compose has
# no build-time symptom, only a bare 502 once a page asks for it.
smoke-dev:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/smoke-dev.ps1

# Static twin of smoke-dev: proves each gateway proxy target resolves to a
# service in the dev compose file. Needs no running stack, so it can gate
# `make dev` instead of reporting after the fact.
check-parity:
	python ./scripts/check-compose-parity.py

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
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} down

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
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} logs -f --tail=100

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

# Kafka integration suite against a real broker in Testcontainers.
# Needs Docker running; first run pulls apache/kafka:3.9.0 (~400MB).
test-kafka-integration:
	cd backend/gomod/shared && go test -tags=integration ./pkg/kafka/ -v -count=1 -timeout 20m

# Adds every Kafka secret to backend/.env, the single source of truth: one
# SCRAM password per client plus the broker TLS settings.
#
#   make kafka-env                 # add any missing value, keep existing
#   make kafka-env ROTATE=1        # regenerate every Kafka password
#
# Compose reads the file for ${VAR} interpolation via --env-file ${ENV_FILE}.
# Re-run after `make env-prod`/`make env-dev`: those replace the file.
kafka-env:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/gen-kafka-env.ps1 $(if $(ROTATE),-Rotate,)

# Phase 6 item 2: creates one SCRAM user per service on the running broker.
# Run once on a fresh kafka-data volume, BEFORE starting the services: with
# SASL on and no credentials yet, nothing can log in - including this script.
# Idempotent, so re-run it after rotating a password in the deployment env.
kafka-sasl-users:
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} exec -T kafka \
		sh /tmp/kafka-sasl-users.sh kafka:29093

# Phase 6 item 4: grants each service the topics and consumer groups it uses.
# Run AFTER kafka-sasl-users, and BEFORE starting the services: the broker is
# configured to deny by default (KAFKA_ALLOW_EVERYONE_IF_NO_ACL_FOUND=false), so
# an ungranted client fails at startup. Idempotent.
kafka-acls:
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} exec -T kafka \
		sh /tmp/kafka-acls.sh kafka:29093

# Proves the ACLs are enforced, not merely present: each identity must reach the
# topics it owns AND be refused another service's. Passwords come from the
# broker's own environment, so no secrets are passed on the command line.
kafka-acl-verify:
	docker compose -f ${PROD_COMPOSE} --env-file ${ENV_FILE} exec -T kafka \
		sh /tmp/kafka-acl-verify.sh kafka:29093 kafka:29096

# Phase 6 item 3: broker TLS material (CA + broker cert + JKS keystore).
# Writes backend/kafka/tls/; only ca.crt is committed. Run kafka-env after.
kafka-tls-certs:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/gen-kafka-tls-certs.ps1

# Every dead-letter topic should be empty in steady state. A non-zero lag means
# records exhausted their retries and wait for `dlqreplay`.
check-dlq:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/check-dlq.ps1

# Truncates every DLQ topic to offset 0 without dropping the topic (and its ACLs).
# Destroys the dead letters: inspect them before running this.
clear-dlq:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/clear-dlq.ps1

# Health of the six Kafka consumer services, straight through the edge.
kafka-health:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/check-kafka-health.ps1

# Rewinds one consumer group's offsets. Debug only: replaying side-effecting
# consumers duplicates notifications and emails.
#   make kafka-reset-group GROUP=goshop.notification_service_events_v3 MODE=Latest
kafka-reset-group:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/kafka-reset-group.ps1 -Group "$(GROUP)" -Mode "$(or $(MODE),Latest)" $(if $(DATETIME),-DateTime "$(DATETIME)",)

# Ad-hoc Grafana Cloud Loki query (needs GRAFANA_LOKI_READ_TOKEN in backend/.env).
#   make logq Q='{container="goshop-order"}' HOURS=6
# Make strips bare double quotes, so $-escape them in the query when passing
# one through a shell: Q='{container=$\"goshop-order$\"}'. Simpler queries
# without quotes work directly.
logq:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/logq.ps1 '$(Q)' -Hours $(or $(HOURS),2) -Limit $(or $(LIMIT),10)

.PHONY: importdb exportdb server worker ws-gateway identity notification payment coupon inventory catalog order chat media build-identity build-notification build-payment build-coupon build-inventory build-catalog build-order build-chat build-media migrate-create migrate-up migrate-down sqlc build build-worker build-ws-gateway run-binary prod stop-prod logs-prod bash noapp stop-noapp dev stop-dev dev-host smoke-dev check-parity swagger loadtest stresstest spiketest test-kafka-integration kafka-env kafka-sasl-users kafka-acls kafka-acl-verify kafka-tls-certs check-dlq clear-dlq kafka-health kafka-reset-group logq seed seed-docker env-dev env-prod

