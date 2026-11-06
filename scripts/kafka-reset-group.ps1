# =============================================================================
# GOshop Kafka consumer-group reset (recovery tool).
# Resets a group's offsets to latest (skip a poison backlog), earliest, or a
# timestamp. The group must have no active members: stop the owning service
# first, otherwise the broker refuses the reset.
#
#   make kafka-reset-group GROUP=goshop.notification_service_events_v3
#   make kafka-reset-group GROUP=... MODE=DateTime DATETIME=2026-11-01T00:00:00
# =============================================================================

param(
    [Parameter(Mandatory = $true)]
    [string]$Group,

    [ValidateSet("Latest", "Earliest", "DateTime")]
    [string]$Mode = "Latest",

    [string]$DateTime = "",

    [string]$Container = "goshop-kafka",

    [string]$Topic = ""
)

$ErrorActionPreference = "Stop"

# BIN=/opt/kafka/bin: the binary is not on PATH inside apache/kafka images.
# 29093 is the plaintext BOOTSTRAP listener, so no SASL credentials are needed.
$BIN = "/opt/kafka/bin"
$Bootstrap = "localhost:29093"

function Invoke-KafkaGroups {
    param([string[]]$KafkaArgs)
    $joined = ($KafkaArgs | ForEach-Object { if ($_ -match '\s') { "'$_'" } else { $_ } }) -join " "
    docker exec $Container sh -c "$BIN/kafka-consumer-groups.sh --bootstrap-server $Bootstrap $joined" 2>&1
}

Write-Host "Groups:" -ForegroundColor Cyan
Invoke-KafkaGroups @("--list")

$resetArgs = @("--group", $Group, "--reset-offsets", "--execute")
switch ($Mode) {
    "Latest"   { $resetArgs += @("--to-latest") }
    "Earliest" { $resetArgs += @("--to-earliest") }
    "DateTime" {
        if ([string]::IsNullOrWhiteSpace($DateTime)) {
            throw "DateTime mode needs -DateTime 'yyyy-MM-ddTHH:mm:ss'"
        }
        $resetArgs += @("--to-datetime", $DateTime)
    }
}

if ([string]::IsNullOrWhiteSpace($Topic)) {
    $resetArgs += @("--all-topics")
} else {
    $resetArgs += @("--topic", $Topic)
}

Write-Host "`nResetting group '$Group' ($Mode)..." -ForegroundColor Yellow
Invoke-KafkaGroups $resetArgs
Write-Host "Done. Consumers in '$Group' pick up from the new offset on next poll." -ForegroundColor Green
