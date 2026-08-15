include .env
export

MIGRATION_DIR=internal/db/migrations

CONNECTION_STRING=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}

importdb:
	docker exec -i postgres-db psql -U postgres -d master-golang < ./backupdb.sql

exportdb:
	docker exec -i postgres-db pg_dump -U postgres -d master-golang > ./backupdb.sql

server:
	cd cmd/api && go run .

build:
	go build -o bin/goshop ./cmd/api

run-binary:
	./bin/goshop

sqlc:
	sqlc generate

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

.PHONY: importdb exportdb server migrate-create migrate-up migrate-down migrate-force migrate-drop migrate-goto migrate-down-n sqlc build run-binary
