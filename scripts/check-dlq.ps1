# Phase-3 DLQ check: every dead-letter topic should be empty in steady state.
# A non-zero backlog means records exhausted their retries and no consumer will
# pick them up until an operator replays them (cmd/dlqreplay).
#   make check-dlq
#
# Backlog = latest offset - earliest offset, not the latest offset alone:
# truncating a topic (kafka-delete-records, retention) leaves the end offset
# where it was and only advances the log start, so latest-offset alone reports
# a cleared topic as still full.
#
# Reads the unauthenticated bootstrap listener: 29093 in prod (BOOTSTRAP), 29092
# in dev (PLAINTEXT). Both are plaintext and need no credentials, which is the
# point: KAFKA_SASL_ADMIN_* does not travel into this script. 29092 also answers
# in prod, but only over SASL, so 29093 is tried first there.
$ErrorActionPreference = "Stop"
$bin = "/opt/kafka/bin"

function Resolve-Bootstrap {
  # Native stderr must not reach the pipeline: under $ErrorActionPreference =
  # "Stop" PowerShell turns docker's warning on the unauthenticated prod probe
  # into a terminating error before the dev listener is tried.
  $previous = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    foreach ($b in @("kafka:29093", "kafka:29092")) {
      $out = docker exec goshop-kafka $bin/kafka-get-offsets.sh `
        --bootstrap-server $b --topic goshop.order.v1.dlq --time -1 2>$null | Out-String
      if ($out -match ":\d+:\d+") { return $b }
    }
  } finally {
    $ErrorActionPreference = $previous
  }
  throw "kafka: no unauthenticated bootstrap listener answered (tried 29093, 29092)"
}
$bootstrap = Resolve-Bootstrap

$topics = @(
  "order", "order-return", "payment", "user", "email",
  "inventory", "review", "notification", "media", "shipping"
)

# Offset of one partition at a point in time: -2 earliest, -1 latest.
function Get-PartitionOffset([string]$topic, [int]$time) {
  $out = docker exec goshop-kafka $bin/kafka-get-offsets.sh `
    --bootstrap-server $bootstrap --topic $topic --time $time 2>&1 | Out-String
  if ($out -match "does not exist|Exception") { return $null }
  $map = @{}
  foreach ($line in ($out -split "`n")) {
    if ($line -match '^[^:]+:(\d+):(\d+)\s*$') { $map[[int]$Matches[1]] = [long]$Matches[2] }
  }
  return $map
}

$failed = 0
foreach ($d in $topics) {
  $topic = "goshop.$d.v1.dlq"
  $earliest = Get-PartitionOffset $topic -2
  if ($null -eq $earliest) {
    Write-Host "MISSING  $topic"
    $failed++
    continue
  }
  $latest = Get-PartitionOffset $topic -1

  $backlog = 0
  foreach ($p in $earliest.Keys) {
    $end = if ($latest.ContainsKey($p)) { $latest[$p] } else { $earliest[$p] }
    $backlog += [Math]::Max(0, $end - $earliest[$p])
  }

  if ($backlog -gt 0) {
    Write-Host "NOT EMPTY $topic - $backlog record(s) need replay"
    $failed++
  } else {
    Write-Host "EMPTY    $topic"
  }
}

if ($failed -gt 0) {
  Write-Host ""
  Write-Host "$failed topic(s) need attention. Inspect headers first:"
  Write-Host "  docker exec goshop-kafka $bin/kafka-console-consumer.sh --bootstrap-server $bootstrap --topic <dlq> --from-beginning --max-messages 1 --property print.headers=true"
  Write-Host "Then replay after fixing the cause:"
  Write-Host "  go run ./cmd/dlqreplay -all    (from backend/gomod)"
  Write-Host "Or discard them:  make clear-dlq"
  exit 1
}
Write-Host "All DLQ topics empty."
