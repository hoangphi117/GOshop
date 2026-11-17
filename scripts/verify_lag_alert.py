"""Prove ConsumerLagHigh actually fires — no broker, no real lag.

Approach: a throwaway scrape target serves a synthetic
kafka_consumergroup_lag=5000 in Prometheus exposition format; a throwaway
Prometheus loads the *real* rule file from system/prometheus/rules and evaluates
it with a 2s scrape / 5s evaluation interval. ~60s, fully hermetic.

Both containers run as processes inside one prom/prometheus container (the image
ships busybox httpd and wget). The scratch config is written to /tmp so the
committed prometheus.yml and rule files are used unmodified except for the
rule_files path, which is mounted read-only at /etc/prometheus/rules.

Run from the repo root: python scripts/verify_lag_alert.py
Exit code non-zero if the alert never reaches firing.
"""

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
RULES = ROOT / "system" / "prometheus" / "rules"
ALERT = "ConsumerLagHigh"

SCRIPT = r"""
set -e
mkdir -p /tmp/scrape /tmp/data /tmp/www
cat > /tmp/scrape/metrics <<'EOF'
kafka_consumergroup_lag{consumergroup="goshop.payment.succeeded",topic="goshop.payment.v1",partition="0"} 5000
EOF
httpd -p 9109 -h /tmp/scrape

cat > /tmp/p.yml <<'EOF'
global:
  scrape_interval: 2s
  evaluation_interval: 5s
rule_files:
  - /etc/prometheus/rules/*.yml
scrape_configs:
  - job_name: synthetic
    # busybox httpd sends no Content-Type; Prometheus 3 refuses blank types
    # unless the protocol is declared up front.
    fallback_scrape_protocol: PrometheusText0.0.4
    static_configs:
      - targets: ["127.0.0.1:9109"]
EOF

prometheus --config.file=/tmp/p.yml --storage.tsdb.path=/tmp/data >/tmp/log 2>&1 &
sleep 8
wget -qO- http://127.0.0.1:9090/-/ready
echo
sleep 20
wget -qO- http://127.0.0.1:9090/api/v1/alerts
"""


def main() -> int:
    out = subprocess.run(
        [
            "docker", "run", "--rm", "--name", "goshop-lag-alert-test",
            "-v", f"{RULES}:/etc/prometheus/rules:ro",
            "--entrypoint", "sh", "prom/prometheus:v3.6.0", "-c", SCRIPT,
        ],
        capture_output=True, text=True, encoding="utf-8", errors="replace",
    )
    body = out.stdout
    start = body.find('{"status"')
    if start < 0:
        print((out.stdout + out.stderr)[-2000:])
        print("FAIL no API response")
        return 1

    alerts = json.loads(body[start:])["data"]["alerts"]
    fired = [a for a in alerts if a["labels"].get("alertname") == ALERT and a["state"] == "firing"]
    if not fired:
        states = ", ".join(f'{a["labels"].get("alertname")}={a["state"]}' for a in alerts) or "no alerts"
        print(f"FAIL {ALERT} did not reach firing ({states})")
        return 1

    a = fired[0]
    print(f"OK {ALERT} firing for {a['labels'].get('consumergroup')} "
          f"on {a['labels'].get('topic')} (value {a.get('value')})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
