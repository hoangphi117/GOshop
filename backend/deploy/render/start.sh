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

start() {
  name="$1"; shift
  echo "start $name: $*"
  /app/bin/"$name" &
}

# Every service opens its own pool and runs its own migrations at boot. Firing
# all seventeen at once means seventeen connection bursts against the same
# database, and the loser dies with
#   pq: remaining connection slots are reserved for roles with the SUPERUSER attribute
# which showed up as a random service (notification and coupon last time)
# failing while its neighbours booted fine. Starting them one at a time, waiting
# for the listener before the next, removes the race at the cost of boot time.
start_serial() {
  name="$1"; port="$2"
  echo "start $name on :$port"
  /app/bin/"$name" &
  pid=$!
  i=0
  while [ $i -lt 40 ]; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "  $name exited during boot; see its log above"
      return 0
    fi
    if netstat -tln 2>/dev/null | grep -q ":$port "; then
      echo "  $name listening"
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  echo "  $name did not listen on :$port within 40s; continuing"
}

# Postgres/Redis/Kafka come from managed providers; each service runs its own
# migrations at boot, so ordering below is about listeners, not schema.
#
# Every service opens its pool before it migrates, and golang-migrate takes its
# own connection on top of that. On a session-limited database (Supabase free is
# ~15 session-mode slots) seventeen of those in one container is guaranteed to
# run out, and the services that lose the race die with
#   pq: remaining connection slots are reserved for roles with the SUPERUSER attribute
# Set RUN_MIGRATIONS=false once the schema is in place: the pool is then the
# only connection each service holds, and the stack fits.
if [ "${RUN_MIGRATIONS:-true}" = "false" ]; then
  echo "RUN_MIGRATIONS=false: skipping per-service schema migrations"
  export SKIP_MIGRATIONS=true
fi

# Topics first: Redpanda Cloud does not auto-create, and a producer that starts
# before its topic exists fails the event it was publishing. Non-fatal because
# a re-run is idempotent and a broker outage should not hide the service log.
if [ -n "${KAFKA_BROKERS:-}" ] && [ -x /app/bin/kafkatopics ]; then
  echo "bootstrap kafka topics"
  /app/bin/kafkatopics 2>&1 | tail -3 || echo "kafka topic bootstrap failed; continuing"
fi

# Redis probe. Every service already reports "write: broken pipe" on failure,
# but that text covers three different causes: a TLS listener answering
# plaintext, credentials, and a connection budget that was already spent.
# One raw PING before the services boot tells them apart, using netcat from
# the image instead of adding a diagnostic binary.
if [ -n "${REDIS_URL:-}" ]; then
  echo "probe redis"
  hostport=${REDIS_URL#*://}
  hostport=${hostport##*@}
  hostport=${hostport%%/*}
  host=${hostport%%:*}
  port=${hostport##*:}
  case "$REDIS_URL" in
    rediss://*) echo "  REDIS_URL scheme is rediss (TLS) using port $port" ;;
    *) echo "  REDIS_URL scheme is plaintext using port $port" ;;
  esac
  # busybox nc has no -q; -w bounds the wait and closing stdin ends the read
  # once the server replies. A PONG on stdout is a healthy instance.
  if printf 'PING\r\n' | nc -w 3 "$host" "$port" 2>&1 | tr -d '\r' | grep -q '^+PONG'; then
    echo "  redis answered PONG"
  else
    echo "  redis did NOT answer PONG; the services will fall back to in-memory"
  fi
fi

start_serial identity 8081
start_serial catalog 8086
start_serial cart 8087
start_serial coupon 8084
start_serial inventory 8090
start_serial payment 8083
start_serial order 8088
start_serial review 8091
start_serial banner 8089
start_serial media 8094
start_serial analytics 8092
start_serial notification 8082
start_serial recommendations 8095
start_serial assistant 8097
start_serial chat 8093
start_serial ws-gateway 8085
start worker

# Last, and in the foreground: the image lives exactly as long as the gateway.
echo "start api on :${PORT:-8080}"
exec /app/bin/api
