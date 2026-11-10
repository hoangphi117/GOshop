"""Rule-level regression checks for system/prometheus/rules/goshop.yml.

promtool only proves the YAML and PromQL parse. These checks pin the contracts
the dashboards and Alertmanager depend on:
  - every rule carries severity + service labels (grouping / Discord colouring)
  - ConsumerLagHigh uses the same threshold as shared/pkg/kafka.LagAlertThreshold
  - every referenced metric exists (typo = always-silent alert), guarded by the
    expected-metric inventory below
  - metric reference list matches the names exposed by kafka-exporter and our
    own /metrics surface

Run: python scripts/check_alert_rules.py
Exit code is non-zero on any violation, so it can be a CI gate.

The reference inventory is deliberately static: kafka-exporter's metric set is
pinned to the version in docker-compose, and reading it at run time would make
this check pass vacuously when the exporter is down. Regenerate it (see
EXPECTED_* below) when bumping the exporter image.
"""

import re
import sys
from pathlib import Path

RULE_FILE = Path(__file__).resolve().parent.parent / "system" / "prometheus" / "rules" / "goshop.yml"
LAG_THRESHOLD_GO = Path(__file__).resolve().parent.parent / "backend" / "gomod" / "shared" / "pkg" / "kafka" / "healthz.go"

# Metrics this repo's /metrics exposes (shared/pkg/kafka/metrics.go, promauto).
OWN_METRICS = {
    "kafka_publish_ok_total",
    "kafka_publish_errors_total",
    "kafka_dlq_total",
    "kafka_consume_errors_total",
    "kafka_records_published_total",
    "kafka_records_missing_event_id_total",
}

# kafka-exporter v1.9.0 (danielqsj), captured from a live scrape.
EXPORTER_METRICS = {
    "kafka_broker_info",
    "kafka_brokers",
    "kafka_consumergroup_current_offset",
    "kafka_consumergroup_current_offset_sum",
    "kafka_consumergroup_lag",
    "kafka_consumergroup_lag_sum",
    "kafka_consumergroup_members",
    "kafka_exporter_build_info",
    "kafka_topic_partition_current_offset",
    "kafka_topic_partition_in_sync_replica",
    "kafka_topic_partition_leader",
    "kafka_topic_partition_leader_is_preferred",
    "kafka_topic_partition_oldest_offset",
    "kafka_topic_partition_replicas",
    "kafka_topic_partition_under_replicated_partition",
    "kafka_topic_partitions",
}

# Prometheus' own `up` series, present for every scrape target.
BUILTIN_METRICS = {"up"}


def parse_rules(text: str):
    """Tiny YAML subset parser: returns [(name, body_lines)] per `- alert:`."""
    rules, cur = [], None
    for line in text.splitlines():
        m = re.match(r"^      - alert:\s*(\S+)\s*$", line)
        if m:
            cur = (m.group(1), [])
            rules.append(cur)
            continue
        if cur is not None:
            cur[1].append(line)
    return rules


def fail(msg: str, errors: list):
    errors.append(msg)


def main() -> int:
    text = RULE_FILE.read_text(encoding="utf-8")
    errors: list[str] = []

    rules = parse_rules(text)
    if not rules:
        fail("no alert rules found; parser or file changed shape", errors)

    known = OWN_METRICS | EXPORTER_METRICS | BUILTIN_METRICS
    for name, body in rules:
        block = "\n".join(body)
        if not re.search(r"^          severity:\s*\w+", block, re.M):
            fail(f"{name}: missing severity label (Alertmanager routing depends on it)", errors)
        if not re.search(r"^          service:", block, re.M):
            fail(f"{name}: missing service label (Alertmanager group_by uses it)", errors)

        # metric references: `metric{...}` or `metric(` or bare word before `[`
        for ident in set(re.findall(r"\b([a-z][a-z0-9_]{2,})\b", block)):
            if ident in {"alert", "expr", "for", "labels", "annotations", "severity", "service",
                         "summary", "description", "runbook_url", "sum", "by", "and", "or",
                         "rate", "increase", "delta", "predict_linear", "job", "instance",
                         "consumergroup", "topic", "mountpoint", "value", "labels"}:
                continue
            if ident.startswith(("kafka_", "go_", "node_", "process_", "promhttp_")) and ident not in known:
                fail(f"{name}: references unknown metric {ident} (silent alert)", errors)

    lag_rule = next((b for n, b in rules if n == "ConsumerLagHigh"), None)
    if lag_rule is None:
        fail("ConsumerLagHigh rule is missing", errors)
    else:
        m = re.search(r">\s*(\d+)", "\n".join(lag_rule))
        if not m:
            fail("ConsumerLagHigh has no numeric threshold", errors)
        else:
            go_src = LAG_THRESHOLD_GO.read_text(encoding="utf-8")
            gm = re.search(r"LagAlertThreshold\s*=\s*(\d+)", go_src)
            if not gm:
                fail("LagAlertThreshold not found in shared/pkg/kafka/healthz.go", errors)
            elif m.group(1) != gm.group(1):
                fail(
                    f"ConsumerLagHigh threshold {m.group(1)} != LagAlertThreshold {gm.group(1)} "
                    "(alert and /healthz must agree)",
                    errors,
                )

    if errors:
        for e in errors:
            print(f"FAIL {e}")
        return 1
    print(f"OK {len(rules)} rules, {len(known)} known metrics")
    return 0


if __name__ == "__main__":
    sys.exit(main())
