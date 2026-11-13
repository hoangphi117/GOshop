"""Asserts for scripts/patch-openapi-spec.py. Run: python scripts/patch-openapi-spec.test.py

`swag` cannot see across the Go workspace, so the script is what keeps the
150+ service-owned operations in the committed spec. These checks pin the two
behaviours that would silently lose them.
"""

import importlib.util
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("patch", HERE / "patch-openapi-spec.py")
patch = importlib.util.module_from_spec(spec)
spec.loader.exec_module(patch)

committed = json.loads(patch.SPEC.read_text(encoding="utf-8"))

# A fresh gateway-only run: three paths, none of them service-owned.
fresh = {
    "info": {"title": "x", "version": "1", "description": "d"},
    "basePath": "/",
    "paths": {"/healthz": {"get": {"summary": "health"}}},
    "definitions": {"utils.APIResponse": {"type": "object"}},
}
merged = patch.adopted_instead_of_wiped(committed, fresh)

assert len(merged["paths"]) >= len(committed["paths"]), "merge dropped committed paths"
for name in patch.SERVICE_OWNED_PATHS:
    assert name in merged["paths"], f"merge dropped service-owned path {name}"
assert "/healthz" in merged["paths"], "merge dropped the fresh gateway path"
assert "Kafka" in merged["info"]["description"], "stale RabbitMQ description survived"
assert "`" not in json.dumps(merged), "backtick would break the docs.go raw string"

# The three vector shelves keep their envelope, not a bare Shelf.
for name in ("/api/v1/recommendations", "/api/v1/products/similar"):
    schema = merged["paths"][name]["get"]["responses"]["200"]["schema"]
    assert schema["$ref"] == "#/definitions/dto.Shelf", f"{name} lost its envelope"

print(f"ok: {len(merged['paths'])} paths, {len(patch.SERVICE_OWNED_PATHS)} service-owned")
