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

if (-not (Test-Path $envFile)) {
  throw "missing $envFile (API_KEY, KAFKA_SASL_*_PASSWORD are required to interpolate)"
}

Write-Host "== stopping the $other stack =="
# Remove by compose label instead of `docker compose ... down`. `down` has to
# interpolate every `${VAR:?}` in the other file, and the values that reach it
# are the ones `make` exported for the target stack - shell env outranks
# --env-file - while the prod file additionally requires
# KAFKA_SSL_TRUSTSTORE_PASSWORD (written by `make kafka-env`). Both turned a
# routine switch into an interpolation error. The label names the containers
# exactly, so no interpolation is needed and a missing secret cannot block
# leaving a stack.
$otherPath = (Resolve-Path $other).Path
$stale = @(docker ps -aq --filter "label=com.docker.compose.project.config_files=$otherPath")
if ($stale.Count -gt 0) {
  docker rm -f @stale | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "failed to remove the $other containers" }
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
