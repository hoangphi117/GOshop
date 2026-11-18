# Creates every GOshop topic (and its .dlq sibling) on a SASL_SSL broker.
#
# The list comes from backend/gomod/shared/pkg/kafka/topics.go, which is the
# single source of truth for topic names. Run it after setting the three
# variables below; re-running is safe (--if-not-exists).
#
#   $env:KAFKA_BROKERS = "xxxx.any.us-east-1.mpx.prd.cloud.redpanda.com:9092"
#   $env:KAFKA_SASL_USER = "..."
#   $env:KAFKA_SASL_PASSWORD = "..."
#   .\scripts\create-kafka-topics.ps1
$ErrorActionPreference = "Stop"

$broker = $env:KAFKA_BROKERS
$user = $env:KAFKA_SASL_USER
$pass = $env:KAFKA_SASL_PASSWORD

if (-not $broker -or -not $user -or -not $pass) {
  throw "Set KAFKA_BROKERS, KAFKA_SASL_USER and KAFKA_SASL_PASSWORD first."
}

$topics = @(
  "goshop.email.v1"
  "goshop.events.v1"
  "goshop.inventory.v1"
  "goshop.media.v1"
  "goshop.notification.v1"
  "goshop.order-return.v1"
  "goshop.order.v1"
  "goshop.payment.v1"
  "goshop.review.v1"
  "goshop.shipping.v1"
  "goshop.user.v1"
)

# kafka-topics reads its credentials from a properties file, not stdin.
$dir = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "kafka-topic-init")
$conf = Join-Path $dir "client.properties"
@"
security.protocol=SASL_SSL
sasl.mechanism=SCRAM-SHA-256
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="$user" password="$pass";
"@ | Set-Content -Path $conf -Encoding ascii

$created = 0
foreach ($t in $topics) {
  foreach ($name in @($t, "$t.dlq")) {
    docker run --rm -v "${dir}:/cfg:ro" confluentinc/cp-kafka:7.6.0 `
      kafka-topics --bootstrap-server $broker --command-config /cfg/client.properties `
      --create --if-not-exists --topic $name --partitions 1 --replication-factor 1 2>&1 |
      ForEach-Object { "  $_" }
    $created++
  }
}

Remove-Item -Recurse -Force $dir
"created/verified $created topics"
