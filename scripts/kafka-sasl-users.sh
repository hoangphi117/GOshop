# Creates one SCRAM user per service on a running broker.
#
#   prod  make kafka-sasl-users
#
# Prod must run this once against a fresh kafka-data volume, BEFORE bringing the
# services up: SCRAM credentials live in the cluster metadata, so with SASL on
# and no user yet nothing can log in — including the client that would create
# the first user. That is what the BOOTSTRAP listener (127.0.0.1 only) is for.
#
# Re-run after any password rotation; it is idempotent.
#
# Passwords come from KAFKA_SASL_<SERVICE>_PASSWORD in the broker container
# environment (compose passes them through). A missing one falls back to a dev
# default and prints a warning.

set -e
K=/opt/kafka/bin
# Address of an unauthenticated listener. Passed in, because the bootstrap
# listener advertises itself differently inside the network than on the host.
B="${1:?usage: kafka-sasl-users.sh <bootstrap-host:port> e.g. kafka:29093}"
admin_user="${KAFKA_SASL_ADMIN_USER:-admin}"
admin_pass="${KAFKA_SASL_ADMIN_PASSWORD:-admin-secret}"

create_user() {
    name="$1"
    pass="$2"
    if [ -z "$pass" ]; then
        pass="dev-only-${name}-password"
        echo "WARN: no password for $name, using the dev default" >&2
    fi
    $K/kafka-configs.sh --bootstrap-server "$B" --alter \
        --entity-type users --entity-name "$name" \
        --add-config "SCRAM-SHA-512=[password=${pass}]" >/dev/null
    echo "user $name ready"
}

# Super user first: it is the one KAFKA_SUPER_USERS names.
create_user "$admin_user" "$admin_pass"

# The broker's own login for its SASL listener (backend/kafka/kafka_server_jaas.conf).
create_user broker "broker-internal"

create_user api         "${KAFKA_SASL_API_PASSWORD}"
create_user identity    "${KAFKA_SASL_IDENTITY_PASSWORD}"
create_user notification "${KAFKA_SASL_NOTIFICATION_PASSWORD}"
create_user payment     "${KAFKA_SASL_PAYMENT_PASSWORD}"
create_user order       "${KAFKA_SASL_ORDER_PASSWORD}"
create_user inventory   "${KAFKA_SASL_INVENTORY_PASSWORD}"
create_user catalog     "${KAFKA_SASL_CATALOG_PASSWORD}"
create_user coupon      "${KAFKA_SASL_COUPON_PASSWORD}"
create_user cart        "${KAFKA_SASL_CART_PASSWORD}"
create_user banner      "${KAFKA_SASL_BANNER_PASSWORD}"
create_user review      "${KAFKA_SASL_REVIEW_PASSWORD}"
create_user analytics   "${KAFKA_SASL_ANALYTICS_PASSWORD}"
create_user chat        "${KAFKA_SASL_CHAT_PASSWORD}"
create_user media       "${KAFKA_SASL_MEDIA_PASSWORD}"
create_user worker      "${KAFKA_SASL_WORKER_PASSWORD}"
# No wsgateway user: the websocket gateway does not touch Kafka (it fans out
# through Redis pub/sub and calls the chat gRPC service directly), so a SCRAM
# user for it would be a credential nobody holds.
