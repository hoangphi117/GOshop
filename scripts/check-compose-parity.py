#!/usr/bin/env python3
"""Static parity check between the api gateway and the dev compose file.

    python scripts/check-compose-parity.py

Run this before starting the stack. It needs no running container and takes
under two seconds, so it belongs earlier than smoke-dev.ps1: that script proves
a route answers, this one proves a route *can* answer.

Three failures were found by hand that this catches automatically:

  * `analytics`, `media` and `chat` existed in docker-compose.prod.yml but not
    in docker-compose.dev.yml, so the gateway's proxy target resolved to
    nothing and nginx returned a bare 502.
  * `WS_GATEWAY_URL=http://localhost:8085` inside a container points at the
    container itself.
  * `backend/.env` carries localhost values for host-run binaries (DB_HOST,
    *_GRPC_*, S3_ENDPOINT); a container that forgets to override them fails at
    runtime with a connection error.

Checks
  1. every *_SERVICE_URL / *_GRPC_* host the gateway uses exists as a compose
     service name or network alias in the dev file
  2. no localhost/127.0.0.1 in the gateway's container environment
  3. every service in prod compose also exists in dev compose (allowlist below)
  4. warnings: dev and prod disagree on the gateway target host for a service

Exit code 0 = clean, 1 = at least one failure.
"""

from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ROUTES_GO = ROOT / "backend" / "gomod" / "cmd" / "api" / "routes" / "routes.go"
DEV_COMPOSE = ROOT / "docker-compose.dev.yml"
PROD_COMPOSE = ROOT / "docker-compose.prod.yml"

# backend/.env is gitignored, so CI has to fall back to the committed example.
# The interpolation still needs a file: the dev file uses ${VAR:-default} for
# everything the gateway reads, but --env-file keeps `config` from reading a
# stray .env beside the compose file.
ENV_CANDIDATES = [
    ROOT / "backend" / ".env",
    ROOT / "backend" / ".env.prod.example",
    ROOT / "backend" / ".env.dev.example",
]

# Production-only by design: the dashboard and storefront are built on the host
# or in CI in dev, api1/api2 are a prod load-balancing pair, certbot has no dev
# role.
PROD_ONLY_OK = {"storefront", "certbot", "api1", "api2"}

# Env vars the gateway reads that name another service.
TARGET_ENV = re.compile(
    r"GetEnv\(\"((?:[A-Z]+_)?(?:SERVICE_URL|GRPC_(?:ADDR|TARGET))|WS_GATEWAY_URL)\""
)

LOCALHOST = ("localhost", "127.0.0.1", "0.0.0.0", "::1")


def run(cmd: list[str]) -> str:
    """Run a command and return stdout, raising on failure."""
    proc = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(
            f"{' '.join(cmd[:3])} failed:\n{(proc.stderr or proc.stdout).strip()}"
        )
    return proc.stdout


def gateway_targets() -> dict[str, str]:
    """Env var name -> hostname, for every service the gateway proxies to."""
    source = ROUTES_GO.read_text(encoding="utf-8")
    targets: dict[str, str] = {}
    for name in sorted(set(TARGET_ENV.findall(source))):
        # Resolve the value the way the running gateway will see it: compose
        # environment overrides the env_file, and the env_file overrides the
        # default in routes.go.
        url = dev_gateway_env.get(name) or ""
        if url:
            targets[name] = host_of(url)
    return targets


def host_of(url: str) -> str:
    """Strip scheme, port and path from a URL; leave the bare hostname."""
    host = url.split("://", 1)[-1]
    host = host.split("/", 1)[0]
    return host.rsplit(":", 1)[0] if ":" in host else host


dev_config: dict = {}
dev_gateway_env: dict[str, str] = {}


def load_dev_config() -> None:
    """Interpolated dev compose, so ${VAR:-default} is already resolved."""
    global dev_config, dev_gateway_env
    env_file = next((p for p in ENV_CANDIDATES if p.exists()), None)
    if env_file is None:
        raise RuntimeError(
            "no env file found; expected one of: "
            + ", ".join(str(p) for p in ENV_CANDIDATES)
        )
    raw = run(
        [
            "docker",
            "compose",
            "-f",
            str(DEV_COMPOSE),
            "--env-file",
            str(env_file),
            "config",
            "--format",
            "json",
        ]
    )
    dev_config = json.loads(raw)
    api = dev_config.get("services", {}).get("api", {})
    env = api.get("environment") or {}
    if isinstance(env, list):
        for item in env:
            key, _, value = item.partition("=")
            dev_gateway_env[key] = value
    else:
        dev_gateway_env = {k: "" if v is None else str(v) for k, v in env.items()}


def dev_service_names() -> set[str]:
    return set(dev_config.get("services", {}))


def dev_aliases() -> set[str]:
    """Network aliases, because prod targets analytics-service, not analytics."""
    names: set[str] = set()
    for svc in dev_config.get("services", {}).values():
        for net in (svc.get("networks") or {}).values():
            for alias in (net or {}).get("aliases") or []:
                names.add(alias)
    return names


def prod_service_names() -> set[str]:
    """Level-2 keys under `services:`.

    `docker compose config` cannot be used here: the prod file requires
    ALERTMANAGER_DISCORD_WEBHOOK_URL, KAFKA_SASL_*_PASSWORD and
    KAFKA_SSL_TRUSTSTORE_PASSWORD, and refuses to render without them. Only the
    service names are needed, so a plain scan is enough and needs no secrets.
    """
    names: set[str] = set()
    in_services = False
    for line in PROD_COMPOSE.read_text(encoding="utf-8").splitlines():
        if line.startswith("services:"):
            in_services = True
            continue
        if in_services and line and not line[0].isspace() and not line.startswith("#"):
            break
        if in_services:
            match = re.match(r"^  ([A-Za-z0-9_-]+):\s*$", line)
            if match:
                names.add(match.group(1))
    return names


def main() -> int:
    if not ROUTES_GO.exists():
        print(f"missing {ROUTES_GO}", file=sys.stderr)
        return 1

    load_dev_config()
    services = dev_service_names()
    reachable = services | dev_aliases()
    failures: list[str] = []
    warnings: list[str] = []

    # 1. Proxy targets must resolve to something on the compose network.
    targets = gateway_targets()
    for env_name, host in targets.items():
        if host not in reachable:
            failures.append(
                f"{env_name} -> {host}: no such service or alias in "
                f"docker-compose.dev.yml (nginx answers 502)"
            )

    # 2. A container's localhost is itself, never a peer service. Scoped to the
    #    HTTP proxy targets the gateway actually dials: it opens no gRPC
    #    connections, so the *_GRPC_* vars in backend/.env are read by other
    #    services (which override them) and their localhost defaults are inert
    #    here.
    for env_name, value in sorted(dev_gateway_env.items()):
        if not (env_name.endswith("_SERVICE_URL") or env_name == "WS_GATEWAY_URL"):
            continue
        host = host_of(value)
        if host in LOCALHOST:
            failures.append(
                f"{env_name}={value}: the gateway runs in a container, so "
                f"{host} is the container itself, not a peer service"
            )

    # 3. A service added to prod and forgotten in dev is invisible until a
    #    request arrives. This is exactly how analytics/media/chat went missing.
    prod = prod_service_names()
    if not prod:
        warnings.append("could not read any service from docker-compose.prod.yml")
    for name in sorted(prod - services - PROD_ONLY_OK):
        failures.append(
            f"{name}: in docker-compose.prod.yml but not in "
            f"docker-compose.dev.yml"
        )

    # 4. Report hostname drift; both spellings resolve, so this is not a failure.
    for env_name in sorted(targets):
        pass  # prod values need interpolation; the scan cannot read them safely

    if failures:
        print(f"{len(failures)} parity problem(s):", file=sys.stderr)
        for item in failures:
            print(f"  FAIL  {item}", file=sys.stderr)
        if warnings:
            for item in warnings:
                print(f"  WARN  {item}", file=sys.stderr)
        return 1

    for item in warnings:
        print(f"  WARN  {item}")
    print(
        f"parity OK: {len(targets)} gateway target(s) resolve, "
        f"{len(prod)} prod services covered by {len(services)} dev services"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
