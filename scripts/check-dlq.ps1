# Phase-3 DLQ check: every dead-letter topic should be empty in steady state.
# A non-zero lag means records exhausted their retries and no consumer will
# pick them up until an operator replays them (cmd/dlqreplay).
#   powershell -ExecutionPolicy Bypass -File scripts/check-dlq.ps1
$ErrorActionPreference = "Stop"

$topics = @(
  "order", "order-return", "payment", "user", "email",
  "inventory", "review", "notification", "media", "shipping"
)

$failed = 0
foreach ($d in $topics) {
  $topic = "goshop.$d.v1.dlq"
  $out = docker exec goshop-kafka /opt/kafka/bin/kafka-get-offsets.sh `
    --bootstrap-server kafka:29092 --topic $topic 2>&1 | Out-String
  if ($out -match "does not exist|error") {
    Write-Host "MISSING  $topic"
    $failed++
    continue
  }
  $total = 0
  foreach ($line in ($out -split "`n")) {
    if ($line -match ':\s*\d+:\s*(\d+)\s*$') { $total += [int]$Matches[1] }
  }
  if ($total -gt 0) {
    Write-Host "NOT EMPTY $topic - $total record(s) need replay"
    $failed++
  } else {
    Write-Host "EMPTY    $topic"
  }
}

if ($failed -gt 0) {
  Write-Host ""
  Write-Host "$failed topic(s) need attention. Inspect headers first:"
  Write-Host "  docker exec goshop-kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server kafka:29092 --topic <dlq> --from-beginning --max-messages 1 --property print.headers=true"
  Write-Host "Then replay after fixing the cause:"
  Write-Host "  go run ./cmd/dlqreplay -all    (from backend/gomod)"
  exit 1
}
Write-Host "All DLQ topics empty."
