#!/bin/sh
# Starts every binary in /app/bin and keeps the container alive on the gateway.
#
# Process supervision is deliberately minimal: if any service dies, the
# container keeps running and Render's /healthz check reports the gateway only.
# That is the ceiling of this approach — see "Trade-off" in
# docs/DEPLOY-DOMAIN-RENDER-VERCEL.md §2.
set -eu

# Internal services listen on the ports the compose file uses, so a local
# `docker compose` run and this image agree on every *_SERVICE_URL.
#
# Single container, so the defaults collide: every gRPC client default is
# localhost, but all of them share this network namespace. The listeners are
# pinned to the compose ports and the clients are re-pointed at 127.0.0.1
# explicitly - a name like "catalog-service" does not resolve here.
export IDENTITY_GRPC_PORT=50051
export CATALOG_GRPC_PORT=50055
export INVENTORY_GRPC_PORT=50054
export COUPON_GRPC_PORT=50053
export CART_GRPC_PORT=50056
export ORDER_GRPC_PORT=50057
export CHAT_GRPC_PORT=50058
export PAYMENT_GRPC_PORT=50052

export IDENTITY_GRPC_ADDR=${IDENTITY_GRPC_ADDR:-127.0.0.1:50051}
export CATALOG_GRPC_ADDR=${CATALOG_GRPC_ADDR:-127.0.0.1:50055}
export CATALOG_GRPC_TARGET=${CATALOG_GRPC_TARGET:-127.0.0.1:50055}
export INVENTORY_GRPC_ADDR=${INVENTORY_GRPC_ADDR:-127.0.0.1:50054}
export COUPON_GRPC_ADDR=${COUPON_GRPC_ADDR:-127.0.0.1:50053}
export CART_GRPC_ADDR=${CART_GRPC_ADDR:-127.0.0.1:50056}
export ORDER_GRPC_ADDR=${ORDER_GRPC_ADDR:-127.0.0.1:50057}
export ORDER_GRPC_TARGET=${ORDER_GRPC_TARGET:-127.0.0.1:50057}

# Every service except the gateway shares this process environment, so one
# exported SERVER_PORT would move all of them onto the same listener. Only
# identity needs a pin, and it reads IDENTITY_SERVER_PORT.
export IDENTITY_SERVER_PORT=${IDENTITY_SERVER_PORT:-8081}

# Compose file hostnames do not resolve in a single container. Every client
# default is already 127.0.0.1 in code; this covers the values a deployment
# copied out of docker-compose.prod.yml, which point at *-service names.
for v in IDENTITY_SERVICE_URL NOTIFICATION_SERVICE_URL CATALOG_SERVICE_URL \
         CATALOG_BASE_URL ASSISTANT_BASE_URL PAYMENT_WEBHOOK_BASE_URL; do
  eval "cur=\${$v:-}"
  case "$cur" in
    *-service*) eval "export $v=http://127.0.0.1:\${cur##*:}" ;;
  esac
done
for v in IDENTITY_GRPC_ADDR CATALOG_GRPC_ADDR CATALOG_GRPC_TARGET \
         INVENTORY_GRPC_ADDR COUPON_GRPC_ADDR CART_GRPC_ADDR \
         ORDER_GRPC_ADDR ORDER_GRPC_TARGET; do
  eval "cur=\${$v:-}"
  case "$cur" in
    *-service*) eval "export $v=127.0.0.1:\${cur##*:}" ;;
  esac
done

start() {  name="$1"; shift
  echo "start $name: $*"
  /app/bin/"$name" &
}

# Postgres/Redis/Kafka come from managed providers; each service runs its own
# migrations at boot, so ordering below is about listeners, not schema.

# Topics first: Redpanda Cloud does not auto-create, and a producer that starts
# before its topic exists fails the event it was publishing. Non-fatal because
# a re-run is idempotent and a broker outage should not hide the service log.
if [ -n "${KAFKA_BROKERS:-}" ] && [ -x /app/bin/kafkatopics ]; then
  echo "bootstrap kafka topics"
  /app/bin/kafkatopics 2>&1 | tail -3 || echo "kafka topic bootstrap failed; continuing"
fi

start identity
start catalog
start cart
start coupon
start inventory
start payment
start order
start review
start banner
start media
start analytics
start notification
start recommendations
start assistant
start chat
start ws-gateway
start worker

# Last, and in the foreground: the image lives exactly as long as the gateway.
echo "start api on :${PORT:-8080}"
exec /app/bin/api
