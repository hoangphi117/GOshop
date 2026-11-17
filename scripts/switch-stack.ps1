# Switches the local Docker stack between dev and prod definitions.
#
#   powershell -File scripts/switch-stack.ps1 -Stack prod
#   powershell -File scripts/switch-stack.ps1 -Stack dev -Force
#
# Why a script and not `docker compose up -d`: both files use the same
# container_name values and the same network, so containers from the previous
# stack survive `up -d` and the result is a half-dev/half-prod mix. That mix is
# what fed the Kafka outage in Nov 2026: prod services dialled SASL_SSL :29096
# while `goshop-kafka` was still the dev plaintext broker.
#
# The switch is destructive for the stack being replaced (containers are
# removed, named volumes are kept), hence -Force for prod.

param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("dev", "prod")]
  [string]$Stack,

  [switch]$Force
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$target = if ($Stack -eq "prod") { "docker-compose.prod.yml" } else { "docker-compose.dev.yml" }
$other = if ($Stack -eq "prod") { "docker-compose.dev.yml" } else { "docker-compose.prod.yml" }
$envFile = "./backend/.env"
# The prod file interpolates with `${VAR:?…}` for every SASL password, the
# internal token and the broker keystore secret. `make env-dev` copies the dev
# variant over backend/.env, which has none of them, so `down` on the prod stack
# then fails to interpolate and the switch aborts. Tear-down only needs the
# project/container labels, so point interpolation at the prod variant instead.
$otherEnvFile = if ($Stack -eq "dev" -and (Test-Path "./backend/.env.prod")) {
  "./backend/.env.prod"
} else {
  $envFile
}

if (-not (Test-Path $envFile)) {
  throw "missing $envFile (API_KEY, KAFKA_SASL_*_PASSWORD are required to interpolate)"
}

Write-Host "== stopping the $other stack =="
docker compose -f $other --env-file $otherEnvFile down --remove-orphans
if ($LASTEXITCODE -ne 0) {
  # A missing prod secret must not block leaving prod. `down` only needs the
  # container labels, so fall back to the project namespace.
  Write-Host "down via env file failed; removing containers by project label" -ForegroundColor Yellow
  docker ps -aq --filter "label=com.docker.compose.project.config_files=$((Resolve-Path $other).Path)" |
    ForEach-Object { docker rm -f $_ }
  if ($LASTEXITCODE -ne 0) { throw "down failed for $other" }
}

Write-Host "== starting the $Stack stack =="
# Both compose files tag their images `shopping-<service>`, so whichever stack
# built last owns those tags. Without a build here, switching to dev would run
# the prod images (entrypoint /app/service) and every service dies with
# `exec: "/app/service": no such file or directory`. Rebuilding is the price of
# sharing image tags; Docker's layer cache keeps it to seconds when nothing
# changed.
Write-Host "== building $Stack images (cached layers make a no-op fast) =="
docker compose -f $target --env-file $envFile build
if ($LASTEXITCODE -ne 0) { throw "build failed for $target" }

# --force-recreate: compose does not rebuild a container whose config hash it
# has not recomputed, which is how a production service kept running a stale
# environment for days.
docker compose -f $target --env-file $envFile up -d --force-recreate
if ($LASTEXITCODE -ne 0) { throw "up failed for $target" }

if ($Stack -eq "prod") {
  # A fresh kafka-data volume has no SCRAM users and no ACLs, and the prod broker
  # denies by default: every client would fail to authenticate and `up` would
  # still print "Started". Both scripts are idempotent, so running them on an
  # already-bootstrapped volume is a no-op (a few seconds of broker round-trips).
  Write-Host "== prod: bootstrapping SCRAM users and ACLs (idempotent) =="
  docker compose -f $target --env-file $envFile exec -T kafka sh /tmp/kafka-sasl-users.sh kafka:29093
  if ($LASTEXITCODE -ne 0) { throw "kafka SCRAM user bootstrap failed" }
  docker compose -f $target --env-file $envFile exec -T kafka sh /tmp/kafka-acls.sh kafka:29093
  if ($LASTEXITCODE -ne 0) { throw "kafka ACL grant failed" }

  Write-Host "== prod: verifying broker ACLs (21 checks) =="
  # Idempotent; proves each service identity can still reach its topics and is
  # refused another service's.
  docker compose -f $target --env-file $envFile exec -T kafka sh /tmp/kafka-acl-verify.sh kafka:29093 kafka:29096
  if ($LASTEXITCODE -ne 0) { throw "kafka ACL verification failed" }
}

Write-Host "== verifying no service runs stale config =="
python scripts/check-compose-drift.py $(if ($Stack -eq "dev") { "--dev" })
if ($LASTEXITCODE -ne 0) {
  Write-Host "drift detected; re-run with --force-recreate on the listed services" -ForegroundColor Yellow
  exit 1
}

Write-Host "stack is $Stack and consistent" -ForegroundColor Green
