#!/usr/bin/env bash
# Generates the Kafka TLS material that scripts/kafka-acl-ci.sh mounts.
#
#   scripts/kafka-tls-certs-ci.sh
#
# Same artifacts as scripts/gen-kafka-tls-certs.ps1 (CA, broker cert, JKS
# keystore + truststore, ca.crt, key.creds, store.creds) but bash, because the
# CI runner has no PowerShell. Nothing here is committed: the CI broker lives
# for one run.
#
# Unlike the PowerShell version this borrows keytool from the Kafka image only
# if the host has no keytool, so a dev box with a JDK needs no Docker pull.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/backend/kafka/tls"
PASS=changeit
DAYS=3650

mkdir -p "$OUT"
cd "$OUT"

# keytool wrapper: use the host JDK when present.
keytool_run() {
  if command -v keytool >/dev/null 2>&1; then
    keytool "$@"
  else
    docker run --rm -v "$OUT:/w" -w /w --entrypoint keytool apache/kafka:3.9.0 "$@"
  fi
}

echo "==> CA"
openssl req -x509 -newkey rsa:2048 -nodes -days "$DAYS" \
  -keyout ca.key -out ca.crt \
  -subj "/O=goshop/CN=goshop-kafka" >/dev/null 2>&1

echo "==> Broker certificate"
openssl req -newkey rsa:2048 -nodes -keyout kafka.key -out kafka.csr \
  -subj "/O=goshop/CN=kafka" >/dev/null 2>&1
cat > san.cnf <<EOF
subjectAltName=DNS:kafka,DNS:localhost,DNS:goshop-kafka,IP:127.0.0.1
EOF
openssl x509 -req -in kafka.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out kafka.crt -days "$DAYS" -extfile san.cnf >/dev/null 2>&1

echo "==> Keystore (PKCS#12 -> JKS)"
openssl pkcs12 -export -name kafka -inkey kafka.key -in kafka.crt \
  -certfile ca.crt -out kafka.p12 -passout "pass:$PASS" >/dev/null 2>&1
rm -f kafka.keystore.jks
keytool_run -importkeystore -noprompt \
  -srckeystore kafka.p12 -srcstoretype PKCS12 -srcstorepass "$PASS" \
  -destkeystore kafka.keystore.jks -deststoretype JKS -deststorepass "$PASS" \
  -destkeypass "$PASS" >/dev/null 2>&1

echo "==> Truststore"
rm -f kafka.truststore.jks
keytool_run -importcert -noprompt -alias ca -file ca.crt \
  -keystore kafka.truststore.jks -storepass "$PASS" >/dev/null 2>&1

# The image's /etc/kafka/docker/configure reads the passwords from these files.
printf '%s' "$PASS" > key.creds
printf '%s' "$PASS" > store.creds

echo "kafka TLS material ready in $OUT"
