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
start() {
  name="$1"; shift
  echo "start $name: $*"
  /app/bin/"$name" &
}

# Postgres/Redis/Kafka come from managed providers; each service runs its own
# migrations at boot, so ordering below is about listeners, not schema.
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
