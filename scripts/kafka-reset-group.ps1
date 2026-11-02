#Requires -Version 7.0
# =============================================================================
# GOshop Kafka consumer-group reset (R5 recovery tool)
# Resets a group offset to latest (skip poison backlog) or to a datetime.
# Usage:
#   scripts/kafka-reset-group.ps1 -Group goshop.notification.order-events -Mode Latest
#   scripts/kafka-reset-group.ps1 -Group goshop.notification.order-events -Mode DateTime -DateTime "2026-11-01T00:00:00"
# Requires the dev kafka container (goshop-kafka) to be running.
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

function Invoke-KafkaGroups {
    param([string[]]$KafkaArgs)
    $joined = $KafkaArgs -join " "
    docker exec $Container bash -c "kafka-consumer-groups.sh --bootstrap-server localhost:9092 $joined" 2>&1
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
