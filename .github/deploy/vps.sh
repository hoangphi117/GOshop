#!/usr/bin/env bash
# Runs ON the VPS, piped over SSH by the `deploy-vps` job in
# .github/workflows/backend-ci.yml. The host never compiles and never needs a
# Go toolchain: CI already built the exact commit, this script only pulls it.
#
# Requires on the host (one-time, see docs/DEPLOY-VPS.md section 4):
#   docker login ghcr.io -u <user>          # PAT with read:packages
#   backend/.env with REDIS_PASSWORD etc.  # untracked, never touched here
set -euo pipefail

: "${IMAGE_TAG:?IMAGE_TAG (the commit sha to deploy) is required}"

REPO="${REPO:-$HOME/goshop}"
COMPOSE="deploy/vps/docker-compose.vps.yml"
REGISTRY="ghcr.io/hoangphi117/goshop"

cd "$REPO"
# The compose file is part of the deploy, so the working tree follows main.
# backend/.env is gitignored, so `reset --hard` cannot clobber the secrets.
git fetch --depth=1 origin main
git reset --hard FETCH_HEAD

cd backend

# --env-file is explicit on every call: compose only auto-loads `.env` when the
# project has not been recorded with a different --env-file path in
# DOCKER_CONFIG. Revoking that record left `constraint: required variable
# REDIS_PASSWORD is missing` on a host whose .env was perfectly fine.
COMPOSE_ARGS=(--env-file .env -f "$COMPOSE")

# Rollback target: the image the running container is on right now.
PREVIOUS=$(docker inspect --format '{{.Config.Image}}' goshop-backend 2>/dev/null || true)

set_image() { # $1=key $2=value
  if grep -q "^$1=" .env; then
    sed -i "s|^$1=.*|$1=$2|" .env
  else
    printf '%s=%s\n' "$1" "$2" >>.env
  fi
}

set_image GOSHOP_BACKEND_IMAGE "$REGISTRY/backend:$IMAGE_TAG"
set_image GOSHOP_EMBEDDER_IMAGE "$REGISTRY/embedder:$IMAGE_TAG"

# Pull first: if the registry is unreachable or the tag is missing, this aborts
# with the old containers still serving traffic.
docker compose "${COMPOSE_ARGS[@]}" pull backend embedder
docker compose "${COMPOSE_ARGS[@]}" up -d --no-build --remove-orphans

echo "waiting for goshop-backend to report healthy (start_period is 180s)..."
for _ in $(seq 1 60); do
  state=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' goshop-backend 2>/dev/null || echo missing)
  case "$state" in
  healthy)
    echo "deployed $IMAGE_TAG: goshop-backend is healthy"
    docker image prune -f --filter "until=168h" >/dev/null 2>&1 || true
    exit 0
    ;;
  unhealthy)
    echo "goshop-backend is unhealthy (docker logs goshop-backend --tail 100)" >&2
    break
    ;;
  *)
    sleep 5
    ;;
  esac
done

if [ -n "$PREVIOUS" ] && [ "$PREVIOUS" != "$REGISTRY/backend:$IMAGE_TAG" ]; then
  echo "rolling back to $PREVIOUS" >&2
  set_image GOSHOP_BACKEND_IMAGE "$PREVIOUS"
  docker compose "${COMPOSE_ARGS[@]}" up -d --no-build backend || true
fi
exit 1
