#!/usr/bin/env python3
"""Proves no compose service is RUNNING stale config.

    python scripts/check-compose-drift.py            # prod stack (default)
    python scripts/check-compose-drift.py --dev      # dev stack

Why this exists
  Nov 2026: `goshop-kafka` carried the prod compose label but was started from
  the dev definition, so it advertised PLAINTEXT :29092 while every prod
  service dialled SASL_SSL :29096. Docker Compose does not recreate a container
  whose config hash it has not recomputed, so `up -d` looked successful while
  half the stack was dead. Two whole days went into that.

  Every service also sets an explicit `container_name`, so dev and prod share
  container names and network `goshop-network`. Switching stacks without
  `down` leaves the other stack's containers in place, silently mixed.

What it checks, per running container:
  1. the compose file it was created from is the one you are checking
  2. its config hash equals the hash of the current compose file
Anything else is a drift failure. Exit 0 = clean, 1 = drift, 2 = setup error.

Usage in a switch routine: run it AFTER `up -d`, before you believe the stack.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
COMPOSE_FILES = {
    "prod": ROOT / "docker-compose.prod.yml",
    "dev": ROOT / "docker-compose.dev.yml",
}
ENV_CANDIDATES = [
    ROOT / "backend" / ".env",
    ROOT / "backend" / ".env.prod.example",
    ROOT / "backend" / ".env.dev.example",
]

# One-shot jobs legitimately exit after doing their work.
ONE_SHOT = {"kafka-init"}


def run(cmd: list[str]) -> str:
    proc = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(
            f"{' '.join(cmd[:4])} failed:\n{(proc.stderr or proc.stdout).strip()}"
        )
    return proc.stdout


def env_file() -> Path:
    found = next((p for p in ENV_CANDIDATES if p.exists()), None)
    if found is None:
        raise RuntimeError("no env file found next to backend/")
    return found


def expected_hashes(compose: Path) -> dict[str, str]:
    """Service -> config hash, exactly as compose will compare it."""
    raw = run(
        [
            "docker",
            "compose",
            "-f",
            str(compose),
            "--env-file",
            str(env_file()),
            "config",
            "--hash",
            "*",
        ]
    )
    hashes: dict[str, str] = {}
    for line in raw.splitlines():
        parts = line.split()
        if len(parts) == 2:
            hashes[parts[0]] = parts[1]
    return hashes


def running(compose: Path) -> list[dict]:
    """Containers belonging to this compose file.

    Only IDs are taken from the list command: `--format {{json .}}` has been
    observed returning an empty `Names` field, so every detail comes from
    `docker inspect` instead.
    """
    raw = run(
        [
            "docker",
            "ps",
            "-a",
            "--filter",
            f"label=com.docker.compose.project.config_files={compose}",
            "--format",
            "{{.ID}}",
        ]
    )
    return [{"ID": line.strip()} for line in raw.splitlines() if line.strip()]


def labels(container_id: str) -> dict[str, str]:
    """Name and labels in a single inspect call."""
    raw = json.loads(
        run(["docker", "inspect", container_id, "--format", "{{json .}}"])
    )
    return {
        "_name": (raw.get("Name") or "").lstrip("/"),
        **(raw.get("Config", {}).get("Labels") or {}),
    }


def main() -> int:
    stack = "dev" if "--dev" in sys.argv else "prod"
    compose = COMPOSE_FILES[stack]

    try:
        expected = expected_hashes(compose)
        containers = running(compose)
    except RuntimeError as exc:
        print(exc, file=sys.stderr)
        return 2

    failures: list[str] = []
    seen: set[str] = set()

    for entry in containers:
        lbl = labels(entry["ID"])
        service = lbl.get("com.docker.compose.service", "")
        # Name comes from inspect (authoritative), never from `docker ps`.
        name = lbl.get("_name") or entry["ID"][:12]
        seen.add(service)

        if service in ONE_SHOT:
            continue

        want = expected.get(service)
        if want is None:
            failures.append(
                f"{name} ({service}): running from {stack} compose but no longer "
                f"defined there"
            )
            continue

        got = lbl.get("com.docker.compose.config-hash", "")
        if got != want:
            failures.append(
                f"{name} ({service}): config changed since it was created "
                f"({got[:8]} != {want[:8]}) -> recreate it"
            )

    missing = sorted(set(expected) - seen - ONE_SHOT)
    for name in missing:
        failures.append(
            f"{name}: defined in {compose.name} but not running (not started, "
            f"or started from the other stack: container names and the network "
            f"are shared, so a mixed stack is possible)"
        )

    if failures:
        print(f"{len(failures)} drift problem(s) in the {stack} stack:", file=sys.stderr)
        for item in failures:
            print(f"  FAIL  {item}", file=sys.stderr)
        print(
            f"\nfix: docker compose -f {compose.name} --env-file backend/.env "
            f"up -d --force-recreate <service>\n"
            f"     or switch stacks explicitly, see scripts/switch-stack.ps1",
            file=sys.stderr,
        )
        return 1

    print(f"no drift: {len(seen)} {stack} service(s) run the current config")
    return 0


if __name__ == "__main__":
    sys.exit(main())
