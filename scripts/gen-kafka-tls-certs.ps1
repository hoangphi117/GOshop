# Generates the Kafka TLS material under backend/kafka/tls/.
#
# What is produced:
#   ca.crt                the CA every service trusts  (COMMITTED)
#   kafka.crt / kafka.key the broker's certificate, SAN kafka / localhost / 127.0.0.1
#   ca.key                the CA signing key           (never committed)
#   kafka.keystore.jks    the broker's identity, in the format the JVM wants
#   kafka.truststore.jks  the CA, same format
#
# PEM was tried first and dropped: Kafka 3.9's PemParser rejects every layout
# openssl can produce (see the keystore step below). JKS is what was verified
# against a real broker.
#
# The broker reads the JKS files from a bind mount, so nothing secret travels in
# the environment. Only the keystore password does; make kafka-env writes it.
#
# Run once, or again to rotate everything:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/gen-kafka-tls-certs.ps1
#
# Needs openssl and Docker (keytool comes from the apache/kafka image).
#
# ponytail: one 10-year self-signed CA, no intermediate, no CRL. That is the
# right shape for a single-broker stack; add an intermediate and a CRL when the
# cluster spans hosts with different trust policies.

param(
  [string]$Days = "3650",
  # Keystore password. Not a secret worth rotating: the file it protects lives
  # in backend/kafka/tls and is mounted read-only into one container.
  [string]$KeystorePassword = "changeit"
)

$ErrorActionPreference = "Stop"
# openssl writes keygen progress to stderr; that is not an error.
$PSNativeCommandUseErrorActionPreference = $false
$ErrorActionPreference = 'Continue'
$root = Split-Path -Parent $PSScriptRoot
$out = Join-Path $root "backend\kafka\tls"
New-Item -ItemType Directory -Force $out | Out-Null

if (-not $Days) { $Days = "3650" }

$subject = "/CN=goshop-kafka/O=goshop"
# Only the RSA key stays on the host. The broker and services already have
# /etc/kafka/kafka_server_jaas.conf bind-mounted from the repo, so PEM files are
# the cheapest way to give the JVM its identity without a JKS dependency.
$san = @(
  "DNS:kafka",
  "DNS:localhost",
  "DNS:goshop-kafka",
  "IP:127.0.0.1"
) -join ","

Push-Location $out
try {
  # ---- 1. CA -------------------------------------------------------------
  Write-Host "CA..."
  openssl req -x509 -newkey rsa:4096 -sha256 -days $Days -nodes `
    -keyout ca.key -out ca.crt `
    -subj $subject `
    -addext "basicConstraints=critical,CA:TRUE,pathlen:0" `
    -addext "keyUsage=critical,keyCertSign,cRLSign" 2>$null | Out-Null

  # An unrelated CA, used only by the negative integration test to prove that a
  # certificate this stack did not sign is refused.
  openssl req -x509 -newkey rsa:2048 -sha256 -days $Days -nodes `
    -keyout cafake.key -out ca-untrusted.crt `
    -subj "/CN=not-goshop/O=someone-else" `
    -addext "basicConstraints=critical,CA:TRUE" 2>$null | Out-Null
  Remove-Item cafake.key -Force

  # ---- 2. broker key + CSR ----------------------------------------------
  Write-Host "broker key..."
  openssl req -newkey rsa:2048 -sha256 -nodes `
    -keyout kafka.key -out kafka.csr `
    -subj "/CN=kafka/O=goshop" 2>$null | Out-Null

  # Keep PKCS#8 ("BEGIN PRIVATE KEY"). Kafka 3.9's PemParser matches the literal
  # "PRIVATE KEY" immediately after the dashes, so PKCS#1
  # ("BEGIN RSA PRIVATE KEY") fails with
  # "No matching PRIVATE KEY entries in PEM file".

  # extfile, because -addext on `x509 -req` is not available on every openssl 3 build.
  @"
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth,clientAuth
subjectAltName=$san
"@ | Set-Content -Path "kafka.ext" -Encoding ascii

  Write-Host "signing..."
  openssl x509 -req -in kafka.csr -CA ca.crt -CAkey ca.key -CAcreateserial `
    -days $Days -sha256 -extfile kafka.ext -out kafka.crt 2>$null | Out-Null

  Remove-Item kafka.csr, kafka.ext -Force

  # ---- 3. JVM keystore + truststore -------------------------------------
  # Kafka 3.9's PEM loader (DefaultSslEngineFactory$PemParser) regex-finds
  # "PRIVATE KEY" blocks in a way that rejects both multi-line PEM files and
  # PKCS#1 keys, so a PEM keystore cannot be made to work from this repo. JKS is
  # what the broker documents and what was verified end-to-end. keytool only
  # exists in a JDK, so borrow it from the same image the broker runs.
  Write-Host "keystores (via apache/kafka image)..."
  $mount = ($out -replace '\\', '/')
  docker run --rm --entrypoint /bin/bash -v "${mount}:/tls" apache/kafka:3.9.0 -c @"
set -e
cd /tls
openssl pkcs12 -export -in kafka.crt -inkey kafka.key -certfile ca.crt \
  -name kafka -out kafka.p12 -passout pass:$KeystorePassword
keytool -importkeystore -noprompt \
  -srckeystore kafka.p12 -srcstoretype PKCS12 -srcstorepass $KeystorePassword \
  -destkeystore kafka.keystore.jks -deststoretype JKS -deststorepass $KeystorePassword \
  -destkeypass $KeystorePassword 2>/dev/null
keytool -importcert -noprompt -alias ca -file ca.crt \
  -keystore kafka.truststore.jks -storetype JKS -storepass $KeystorePassword 2>/dev/null
rm -f kafka.p12
printf '%s' "$KeystorePassword" > key.creds
printf '%s' "$KeystorePassword" > store.creds
"@ | Out-Null
} finally {
  Pop-Location
}

Write-Host ""
Write-Host "wrote $out"
Get-ChildItem $out | ForEach-Object { "  $($_.Name)  $($_.Length) bytes" }
Write-Host ""
Write-Host "COMMITTED: ca.crt. IGNORED: ca.key, kafka.key, *.jks, ca.srl"
Write-Host "Next: make kafka-env   (adds the keystore password to backend/.env)"
