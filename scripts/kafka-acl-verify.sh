#!/usr/bin/env bash
# Proves the broker actually enforces the ACLs created by kafka-acls.sh.
#
#   make kafka-acl-verify
#
# Depends on the fact that allow.everyone.if.no.acl.found=false: without it
# everything not explicitly denied succeeds and this script proves nothing.
# Read-only apart from one probe record per write check.
#
# Every check asserts two things: the identity reaches the topic it owns, and
# it is refused the topic another service owns. A run that only checks the
# first half stays green while the broker is wide open.

set -u
K=/opt/kafka/bin
# Unauthenticated listener, reachable only inside the container network.
B="${1:-kafka:29093}"
# The listener real clients use; ACLs are enforced here. SASL_SSL, not the
# plain SSL listener: without SASL there is no principal to authorize against.
SB="${2:-kafka:29096}"
TRUST=/etc/kafka/secrets/kafka.truststore.jks

pass=0
fail=0
ok()  { echo "  ok    $*"; pass=$((pass+1)); }
bad() { echo "  FAIL  $*"; fail=$((fail+1)); }

# cfg <user> <password> -> prints a client properties file for that identity.
cfg() {
    f="/tmp/acl-$1.properties"
    {
        echo "security.protocol=SASL_SSL"
        echo "sasl.mechanism=SCRAM-SHA-512"
        echo "sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username=\"$1\" password=\"$2\";"
        echo "ssl.truststore.location=$TRUST"
        echo "ssl.truststore.password=changeit"
        # The cert SAN covers kafka/localhost, so verification would pass, but
        # the probe deliberately skips it: this tests authorization, not TLS.
        echo "ssl.endpoint.identification.algorithm="
    } > "$f"
    echo "$f"
}

# expect_ok / expect_deny take the same shape: <user> <pass> <label> <op> <topic>
run() {
    want="$1"; u="$2"; p="$3"; label="$4"; op="$5"; topic="$6"
    conf="$(cfg "$u" "$p")"
    got="$("$op" "$conf" "$topic")"
    if [ "$got" = "$want" ]; then ok "$label"; else bad "$label (wanted $want, got $got)"; fi
}

# kafka-console-consumer exits 0 even after TOPIC_AUTHORIZATION_FAILED (it logs
# the error and returns), so the exit status cannot tell "denied" from "empty
# topic". Both helpers classify on the tool's own output instead.
produce() {
    out="$(echo probe | $K/kafka-console-producer.sh --bootstrap-server "$SB" \
        --producer.config "$1" --topic "$2" 2>&1)"
    case "$out" in
        *AUTHORIZATION*|*"Not authorized"*|*SaslAuthenticationException*) echo deny ;;
        *) echo ok ;;
    esac
}
consume() {
    # No consumer group id: a fresh random one has no Read ACL, so the join
    # fails with GROUP_AUTHORIZATION_FAILED and the read looks denied even when
    # the topic grant is fine. Kafka assigns the partitions directly without a
    # group, which is exactly the grant this check exercises. Group grants are
    # covered by kafka-acls.sh; this script is about topics.
    out="$($K/kafka-console-consumer.sh --bootstrap-server "$SB" \
        --consumer.config "$1" --topic "$2" --from-beginning \
        --max-messages 1 --timeout-ms 8000 --partition 0 2>&1)"
    case "$out" in
        # A missing topic is NOT a denial. Reported as such the assertion goes
        # green on a topic nobody can read because it does not exist there.
        *UNKNOWN_TOPIC_OR_PARTITION*) echo missing ;;
        *AUTHORIZATION*|*"Not authorized"*|*SaslAuthenticationException*) echo deny ;;
        *) echo ok ;;
    esac
}

# Topics must exist: auto-create is off, so a missing topic fails on existence
# and would look like a denial. Every topic a check touches is listed here,
# dead letters included: the DLQ probes below assert on read/write grants, and
# against a missing topic the broker answers UNKNOWN_TOPIC_OR_PARTITION, which
# the classifier read as "allowed".
for t in goshop.order.v1 goshop.user.v1 goshop.payment.v1 goshop.email.v1 \
         goshop.order.v1.dlq goshop.payment.v1.dlq; do
    $K/kafka-topics.sh --bootstrap-server "$B" --create --if-not-exists \
        --topic "$t" --partitions 1 --replication-factor 1 >/dev/null 2>&1 || true
done

pw() { eval "echo \"\${KAFKA_SASL_$1_PASSWORD:-}\""; }

check() {
    want="$1"; user="$2"; label="$3"; op="$4"; topic="$5"
    upper="$(echo "$user" | tr 'a-z' 'A-Z')"
    p="$(pw "$upper")"
    if [ -z "$p" ]; then
        bad "$label (no KAFKA_SASL_${upper}_PASSWORD in the broker env)"
        return
    fi
    if [ "$op" = write ]; then
        run "$want" "$user" "$p" "$label" produce "$topic"
    else
        run "$want" "$user" "$p" "$label" consume "$topic"
    fi
}

echo "ACL enforcement on $SB"

# Group grants. A consumer needs READ on its group id as well as its topic;
# without it the coordinator answers GROUP_AUTHORIZATION_FAILED and the
# handler never runs, while the producer still reports success. Topic-only
# checks stay green in exactly that case (shipping.order.create, Nov 2026).
checkgrp() {
    want="$1"; user="$2"; label="$3"; group="$4"
    upper="$(echo "$user" | tr 'a-z' 'A-Z')"
    p="$(pw "$upper")"
    if [ -z "$p" ]; then
        bad "$label (no KAFKA_SASL_${upper}_PASSWORD in the broker env)"
        return
    fi
    conf="$(cfg "$user" "$p")"
    out="$($K/kafka-consumer-groups.sh --bootstrap-server "$SB" \
        --command-config "$conf" --describe --group "$group" 2>&1)"
    case "$out" in
        *GroupAuthorization*|*"authorization failure"*|*"not authorized"*) got=deny ;;
        *) got=ok ;;
    esac
    if [ "$got" = "$want" ]; then ok "$label"; else bad "$label (wanted $want, got $got)"; fi
}

echo
echo "-- order groups --"
checkgrp ok   order "joins its shipping consumer group"   goshop.shipping.order.create
checkgrp ok   order "joins its payment.succeeded group"   goshop.payment.succeeded
checkgrp deny order "cannot join another service's group" goshop.order.created

echo
echo "-- notification groups --"
# The whole order -> shipping -> return notification path hangs off this one
# group: if the coordinator refuses the join, HandleEvent never runs and no
# notification is produced for any of them (Nov 2026).
checkgrp ok   notification "joins the topic-events group" goshop.notification_service_events_v3
checkgrp deny notification "cannot join order's group"    goshop.shipping.order.create

echo
echo "-- order --"
check ok   order     "writes its own topic goshop.order.v1"        write goshop.order.v1
check ok   order     "reads payment outcomes"                      read  goshop.payment.v1
check deny order     "cannot read another domain's goshop.user.v1" read  goshop.user.v1
check deny order     "cannot write goshop.payment.v1"              write goshop.payment.v1
check ok   order     "may dead-letter payment.v1"                  write goshop.payment.v1.dlq
# The reader owns the dead letter (it is the handler that fails), so its own
# identity has to be able to open it for triage and replay.
check ok   order     "may read back its payment.v1 dead letters"    read  goshop.payment.v1.dlq

echo
echo "-- dead letters (readers only) --"
check deny payment   "cannot read its own payment.v1 dead letters (order's)" read goshop.payment.v1.dlq
check ok   inventory "may read back its order.v1 dead letters"      read  goshop.order.v1.dlq

echo
echo "-- inventory --"
check ok   inventory "reads order.v1"                              read  goshop.order.v1
check deny inventory "cannot write order.v1"                       write goshop.order.v1

echo
echo "-- cart --"
check ok   cart      "reads order.v1"                              read  goshop.order.v1
check deny cart      "cannot read user.v1"                         read  goshop.user.v1

echo
echo "-- identity --"
check ok   identity  "writes user.v1"                              write goshop.user.v1
check deny identity  "cannot write order.v1"                       write goshop.order.v1

echo
echo "-- worker --"
check ok   worker    "reads email.v1"                              read  goshop.email.v1

echo
echo "-- no credentials --"
# TLS without SASL: proves the listener requires an identity, not just a cert.
# /dev/null would speak PLAINTEXT and get refused for the wrong reason.
{
    echo "security.protocol=SSL"
    echo "ssl.truststore.location=$TRUST"
    echo "ssl.truststore.password=changeit"
    echo "ssl.endpoint.identification.algorithm="
} > /tmp/acl-anon.properties
anon_out="$($K/kafka-console-consumer.sh --bootstrap-server "$SB" --topic goshop.order.v1 \
     --from-beginning --max-messages 1 --timeout-ms 5000 \
     --consumer.config /tmp/acl-anon.properties 2>&1)"
case "$anon_out" in
    *AUTHORIZATION*|*SaslAuthentication*|*"disconnected"*|*"Error processing"*) ok "anonymous TLS client rejected" ;;
    *) bad "anonymous TLS client was allowed" ;;
esac

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
