# Purges every record from the DLQ topics without deleting the topics.
#
#   make clear-dlq
#
# Deleting the topic would drop its ACLs and force a recreate; truncating the
# log keeps both. kafka-delete-records advances the log start to the offset we
# give it, so the payload carries the current END offset per partition.
#
# Idempotent: a topic whose earliest offset already equals its latest is skipped.
#
# Every record is printed first. Dead letters are lost work: replay them
# (cmd/dlqreplay) when they are a real failure, and expect the three "probe"
# records that kafka-acl-verify.sh produces per run (one per Write grant) — those
# are authorization checks, not failures.
#
# Same listener resolution as check-dlq.ps1: 29093 (prod BOOTSTRAP) first, then
# 29092 (dev PLAINTEXT).
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

$purged = 0
foreach ($d in $topics) {
  $topic = "goshop.$d.v1.dlq"
  $earliest = Get-PartitionOffset $topic -2
  if ($null -eq $earliest) { continue }
  $latest = Get-PartitionOffset $topic -1

  $partitions = @()
  foreach ($p in $latest.Keys) {
    $start = if ($earliest.ContainsKey($p)) { $earliest[$p] } else { 0 }
    if ($latest[$p] -gt $start) {
      $partitions += '{"topic":"' + $topic + '","partition":' + $p + ',"offset":' + $latest[$p] + '}'
    }
  }
  if ($partitions.Count -eq 0) { continue }

  # Print before destroying: a dead letter that was never read is lost work.
  # print.offset makes duplicates obvious, and every partition is drained so a
  # multi-partition DLQ shows all of its records.
  Write-Host "records  $topic"
  docker exec goshop-kafka $bin/kafka-console-consumer.sh `
    --bootstrap-server $bootstrap --topic $topic --from-beginning `
    --timeout-ms 5000 --property print.headers=true --property print.offset=true |
    ForEach-Object { "         $_" }

  $tmp = Join-Path $env:TEMP "dlq-clear.json"
  [System.IO.File]::WriteAllText($tmp, '{"partitions":[' + ($partitions -join ",") + '],"version":1}')
  docker cp $tmp "goshop-kafka:/tmp/dlq-clear.json" | Out-Null

  docker exec goshop-kafka $bin/kafka-delete-records.sh `
    --bootstrap-server $bootstrap --offset-json-file /tmp/dlq-clear.json | Out-Null
  Write-Host "cleared  $topic"
  $purged++
}

if ($purged -eq 0) {
  Write-Host "Nothing to clear: every DLQ topic is already empty."
} else {
  Write-Host ""
  Write-Host "Cleared $purged topic(s). Re-run make check-dlq to confirm."
}
