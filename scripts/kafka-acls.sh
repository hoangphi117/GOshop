# Creates one ACL set per Kafka client on a running broker.
#
#   prod  make kafka-acls
#   check .\scripts\kafka-acls.sh kafka:29093 --dry-run
#
# scripts/kafka-sasl-users.sh creates the identities; this file grants them
# rights. Run it AFTER that script and over the unauthenticated bootstrap
# listener, because a user cannot be granted anything by a principal that is
# itself still denied.
#
# The table is derived from the publish/consume calls in backend/gomod/services
# and backend/gomod/cmd. When a service starts using a new topic, add a line or
# it is denied at runtime.
#
# Two rules that are easy to miss:
#   * Every consumer needs READ on its *group*, not just its topic. Without it
#     the coordinator rejects the join with GROUP_AUTHORIZATION_FAILED, which
#     reads like a topic problem but is not.
#   * DLQ rights belong to the consumer of a topic (its handler is the one that
#     fails), so the grant is per reader, not one blanket pattern.
#   * An operator inspects and replays dead letters with a service's own SCRAM
#     identity (kafka-console-consumer --consumer.config, cmd/dlqreplay). Grant
#     Read on the DLQ next to Write, or `make check-dlq` can see a backlog that
#     no credential in backend/.env is allowed to open.
#
# Idempotent: kafka-acls.sh accepts duplicates without error.

set -eu
K=/opt/kafka/bin
B="${1:?usage: kafka-acls.sh <bootstrap-host:port> e.g. kafka:29093}"
shift || true
DRY=""
[ "${1:-}" = "--dry-run" ] && DRY=1

acl() {
    if [ -n "$DRY" ]; then
        echo "kafka-acls $*"
        return 0
    fi
    $K/kafka-acls.sh --bootstrap-server "$B" "$@" >/dev/null
}

# allow <user> <topic> <prefixed|literal> <operation...>
allow() {
    u="$1"; tp="$2"; pat="$3"; shift 3
    for op in "$@"; do
        acl --add --allow-principal "User:$u" --operation "$op" \
            --topic "$tp" --resource-pattern-type "$pat"
    done
}

# grp <user> <group>
grp() { acl --add --allow-principal "User:$1" --operation Read --group "$2"; }

#
# ---- producers ---------------------------------------------------------------
#

allow order     goshop.order.v1        literal  Write Describe
allow order     goshop.order-return.v1 literal  Write Describe
allow order     goshop.shipping.v1     literal  Write Describe

allow payment   goshop.payment.v1      literal  Write Describe

allow identity  goshop.user.v1         literal  Write Describe
allow identity  goshop.email.v1        literal  Write Describe

allow inventory goshop.inventory.v1    literal  Write Describe

allow review    goshop.review.v1       literal  Write Describe
allow review    goshop.notification.v1 literal  Write Describe

allow media     goshop.media.v1        literal  Write Describe

# api is the storefront BFF: it fans events out on behalf of the SPA and reads
# back for admin views. Broad on purpose; narrow it if the BFF stops proxying
# Kafka directly.
allow api       goshop.               prefixed Read Describe Write

#
# ---- consumers ---------------------------------------------------------------
#

allow order     goshop.payment.v1      literal  Read Describe
allow order     goshop.shipping.v1     literal  Read Describe
grp order goshop.payment.succeeded
grp order goshop.payment.expired
# Group id is "goshop." + the queue name (kafka.Broker.Consume), and the
# order service's queue is "shipping.order.create", not "shipping.create".
grp order goshop.shipping.order.create

allow inventory goshop.order.v1        literal  Read Describe
allow inventory goshop.order-return.v1 literal  Read Describe
grp inventory goshop.inventory.order_events
grp inventory goshop.inventory.return_events

allow coupon    goshop.order.v1        literal  Read Describe
grp coupon goshop.coupon.order_events

allow cart      goshop.order.v1        literal  Read Describe
grp cart goshop.order.created

allow catalog   goshop.review.v1       literal  Read Describe
grp catalog goshop.review.rating_updated

allow notification goshop.             prefixed Read Describe
# The queue literal has no "." prefix, but Broker.SubscribeToExchange always
# prepends "goshop." to the sanitized queue ("notification_service_events_v3"),
# so the group the broker actually reads is goshop.notification_service_events_v3.
grp notification goshop.notification_service_events_v3

allow analytics goshop.                prefixed Read Describe
grp analytics goshop.analytics.order_events

allow worker    goshop.email.v1        literal  Read Describe
allow worker    goshop.user.v1         literal  Read Describe
allow worker    goshop.media.v1        literal  Read Describe
grp worker goshop.auth_email_queue
grp worker goshop.user.created
grp worker goshop.user.updated
grp worker goshop.user.deleted
grp worker goshop.media.uploaded

# chat speaks Redis pub/sub, not Kafka: no grant on purpose.

#
# ---- dead letters ------------------------------------------------------------
#
# A consumer of a topic is the one whose handler fails, so it is the one that
# writes that topic's DLQ.
#
allow order       goshop.order.v1.dlq        literal Write Describe Read
allow order       goshop.order-return.v1.dlq literal Write Describe Read
allow order       goshop.shipping.v1.dlq     literal Write Describe Read
allow order       goshop.payment.v1.dlq      literal Write Describe Read
allow payment     goshop.payment.v1.dlq      literal Write Describe
allow review      goshop.review.v1.dlq       literal Write Describe
allow review      goshop.notification.v1.dlq literal Write Describe
allow inventory   goshop.order.v1.dlq        literal Write Describe Read
allow inventory   goshop.order-return.v1.dlq literal Write Describe Read
allow coupon      goshop.order.v1.dlq        literal Write Describe
allow cart        goshop.order.v1.dlq        literal Write Describe
allow catalog     goshop.review.v1.dlq       literal Write Describe
allow worker      goshop.email.v1.dlq        literal Write Describe
allow worker      goshop.user.v1.dlq         literal Write Describe
allow worker      goshop.media.v1.dlq        literal Write Describe

# Fan-out subscribers can dead-letter anything they read.
allow notification goshop.             prefixed Write Describe
allow analytics    goshop.             prefixed Write Describe
allow api          goshop.             prefixed Write Describe

echo "kafka ACLs ready"
