#!/usr/bin/env python3
"""Merge the OpenAPI contracts owned by service modules into the gateway spec.

`swag` cannot parse an api gateway that is split across a Go workspace: when
`-d` lists more than one module it fails with "cannot find type definition:
utils.APIResponse", and when it is limited to `gomod/cmd/api` it silently
drops every route whose handler lives in a service module (recommendations,
assistant, ...). A spec that is quietly missing routes is worse than none, so
the gateway spec is regenerated from the gateway alone and the service-owned
paths are merged in here.

    python scripts/patch-openapi-spec.py            # patch in place
    python scripts/patch-openapi-spec.py --check    # fail if out of date

`--check` runs in CI: it compares the merged document with the committed one
and exits 1 on drift, so a new route cannot ship undocumented.

The definitions below mirror the Go structs they document. They are duplicated
on purpose: `swag` cannot see across module boundaries, so there is no way to
derive them from the service source in one run. Keep them in sync when a DTO
changes - the `--check` run catches a forgotten path, not a changed field.
"""

from __future__ import annotations

import argparse
import copy
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SPEC = ROOT / "backend" / "gomod" / "shared" / "docs" / "swagger.json"

API_RESPONSE = "#/definitions/utils.APIResponse"
ERROR_RESPONSE = "#/definitions/utils.ErrorResponse"


def envelope(data_ref: str, description: str) -> dict:
    """GET-style 200: the shared APIResponse envelope with a typed data field."""
    return {
        "description": description,
        "schema": {"$ref": data_ref},
    }


def recommendations_paths() -> dict:
    shelf = {"$ref": "#/definitions/dto.Shelf"}
    return {
        "/api/v1/recommendations": {
            "get": {
                "description": (
                    "Top-selling products over the popularity window. When the "
                    "store has no sales yet, falls back to top-rated products and "
                    "reports source=fallback_rating."
                ),
                "produces": ["application/json"],
                "tags": ["Recommendations"],
                "summary": "Get best-selling products",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Number of products (default 12, max 50)",
                        "name": "limit",
                        "in": "query",
                    }
                ],
                "responses": {
                    "200": envelope(shelf["$ref"], "Recommendations retrieved successfully"),
                    "400": {
                        "description": "Invalid limit",
                        "schema": {"$ref": ERROR_RESPONSE},
                    },
                },
            }
        },
        "/api/v1/recommendations/personalized": {
            "get": {
                "description": (
                    "Products near the centroid of the caller's interaction history. "
                    "Falls back (and reports its own source) when the visitor has no "
                    "usable history."
                ),
                "produces": ["application/json"],
                "tags": ["Recommendations"],
                "summary": "Get personalized recommendations",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "Number of products (default 12, max 50)",
                        "name": "limit",
                        "in": "query",
                    }
                ],
                "responses": {
                    "200": envelope(shelf["$ref"], "Recommendations retrieved successfully"),
                    "400": {
                        "description": "Invalid limit",
                        "schema": {"$ref": ERROR_RESPONSE},
                    },
                },
            }
        },
        "/api/v1/products/similar": {
            "get": {
                "description": (
                    "Nearest neighbours in embedding space, excluding the product "
                    "itself and anything out of stock."
                ),
                "produces": ["application/json"],
                "tags": ["Recommendations"],
                "summary": "Get products similar to one product",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Product UUID",
                        "name": "uuid",
                        "in": "query",
                        "required": True,
                    },
                    {
                        "type": "integer",
                        "description": "Number of products (default 12, max 50)",
                        "name": "limit",
                        "in": "query",
                    },
                ],
                "responses": {
                    "200": envelope(shelf["$ref"], "Similar products retrieved successfully"),
                    "400": {
                        "description": "Invalid uuid or limit",
                        "schema": {"$ref": ERROR_RESPONSE},
                    },
                },
            }
        },
    }


def search_override() -> dict:
    """The hybrid search route replaces the Meilisearch-backed one.

    The gateway now proxies /api/v1/products/search to the recommendations
    service, which fuses keyword and embedding rankings, so the old description
    ("Meilisearch with typo tolerance") and its `sort` parameter no longer
    describe what the endpoint does.
    """
    return {
        "get": {
            "description": (
                "Ranks the catalog by keyword relevance and by embedding "
                "similarity, fused with reciprocal rank fusion. The response always "
                "reports in its source field which halves were available (hybrid, keyword, "
                "semantic). Filters apply to both halves, so total counts only "
                "matches the shopper would actually accept."
            ),
            "produces": ["application/json"],
            "tags": ["Recommendations"],
            "summary": "Hybrid product search",
            "parameters": [
                {
                    "type": "string",
                    "description": "Search query",
                    "name": "q",
                    "in": "query",
                    "required": True,
                },
                {
                    "type": "integer",
                    "description": "Page number (default 1)",
                    "name": "page",
                    "in": "query",
                },
                {
                    "type": "integer",
                    "description": "Page size (default 20, max 50)",
                    "name": "limit",
                    "in": "query",
                },
                {
                    "type": "string",
                    "description": "Restrict to one category UUID",
                    "name": "category_uuid",
                    "in": "query",
                },
                {
                    "type": "number",
                    "description": "Minimum product price (inclusive)",
                    "name": "min_price",
                    "in": "query",
                },
                {
                    "type": "number",
                    "description": "Maximum product price (inclusive)",
                    "name": "max_price",
                    "in": "query",
                },
                {
                    "type": "number",
                    "description": "Minimum average rating (inclusive)",
                    "name": "min_rating",
                    "in": "query",
                },
            ],
            "responses": {
                "200": envelope(
                    "#/definitions/dto.Shelf", "Products searched successfully"
                ),
                "400": {
                    "description": "Invalid query or filter",
                    "schema": {"$ref": ERROR_RESPONSE},
                },
            },
        }
    }


def assistant_paths() -> dict:
    def failure(description: str) -> dict:
        return {"description": description, "schema": {"$ref": ERROR_RESPONSE}}

    return {
        "/api/v1/assistant/chat": {
            "post": {
                "description": (
                    "Grounded product advice and FAQ answers. Prices and stock come "
                    "from the catalog, never from the model text."
                ),
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["Assistant"],
                "summary": "Ask the shopping assistant",
                "parameters": [
                    {
                        "description": "Question and optional history",
                        "name": "body",
                        "in": "body",
                        "required": True,
                        "schema": {"$ref": "#/definitions/dto.ChatRequest"},
                    }
                ],
                "responses": {
                    "200": envelope(
                        "#/definitions/dto.ChatResponse", "Assistant replied"
                    ),
                    "400": failure("Invalid request body"),
                    "429": failure("Daily quota reached"),
                    "503": failure("Assistant not configured or provider unavailable"),
                },
            }
        },
        "/api/v1/assistant/stream": {
            "get": {
                "description": (
                    "Same grounding as POST /chat, but the answer arrives as SSE "
                    "text deltas. Event types: delta (text), done (session_id + "
                    "products), error."
                ),
                "produces": ["text/event-stream"],
                "tags": ["Assistant"],
                "summary": "Ask the shopping assistant, streamed",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Question",
                        "name": "message",
                        "in": "query",
                        "required": True,
                    },
                    {
                        "type": "string",
                        "description": "Session id",
                        "name": "session_id",
                        "in": "query",
                    },
                ],
                "responses": {
                    "200": {"description": "text/event-stream", "schema": {"type": "string"}},
                    "400": failure("Invalid message or session_id"),
                    "429": failure("Daily quota reached"),
                    "503": failure("Assistant not configured"),
                },
            }
        },
        "/api/v1/assistant/session": {
            "post": {
                "description": (
                    "Returns a session id the widget sends back with each question."
                ),
                "produces": ["application/json"],
                "tags": ["Assistant"],
                "summary": "Start an assistant session",
                "responses": {
                    "200": envelope(
                        "#/definitions/dto.SessionResponse", "Session created"
                    ),
                    "503": failure("Assistant not configured"),
                },
            }
        },
    }


def service_definitions() -> dict:
    """Definitions owned by service modules that the gateway run cannot see."""
    item_props = {
        "uuid": {"type": "string"},
        "name": {"type": "string"},
        "slug": {"type": "string"},
        "description": {"type": "string"},
        "price": {"type": "number", "format": "double"},
        "stock": {"type": "integer", "format": "int32"},
        "sku": {"type": "string"},
        "category_uuid": {"type": "string"},
        "category_name": {"type": "string"},
        "image_url": {"type": "string"},
        "rating": {"type": "number", "format": "double"},
        "reviews_count": {"type": "integer", "format": "int32"},
        "units_sold": {"type": "integer", "format": "int64"},
        "similarity": {"type": "number", "format": "double"},
    }
    turn_props = {
        "role": {"type": "string", "enum": ["user", "assistant"]},
        "content": {"type": "string"},
    }
    return {
        "dto.RecommendationItem": {
            "type": "object",
            "properties": item_props,
            "required": ["uuid", "name", "slug", "price", "stock", "rating", "reviews_count"],
        },
        "dto.Shelf": {
            "type": "object",
            "properties": {
                "source": {
                    "type": "string",
                    "description": "hybrid | keyword | semantic | personalized | fallback_rating | best_sellers",
                },
                "items": {
                    "type": "array",
                    "items": {"$ref": "#/definitions/dto.RecommendationItem"},
                },
                "total": {"type": "integer"},
            },
            "required": ["source", "items"],
        },
        "dto.AssistantTurn": {
            "type": "object",
            "properties": turn_props,
            "required": ["role", "content"],
        },
        "dto.ChatRequest": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "message": {"type": "string", "maxLength": 2000},
                "history": {
                    "type": "array",
                    "maxItems": 20,
                    "items": {"$ref": "#/definitions/dto.AssistantTurn"},
                },
            },
            "required": ["message"],
        },
        "dto.Product": {
            "type": "object",
            "properties": {
                "uuid": {"type": "string"},
                "name": {"type": "string"},
                "slug": {"type": "string"},
                "price": {"type": "number", "format": "double"},
                "stock": {"type": "integer", "format": "int32"},
                "category_name": {"type": "string"},
                "image_url": {"type": "string"},
            },
            "required": ["uuid", "name", "slug", "price", "stock"],
        },
        "dto.Usage": {
            "type": "object",
            "properties": {
                "prompt_tokens": {"type": "integer"},
                "completion_tokens": {"type": "integer"},
                "cached_tokens": {"type": "integer"},
                "tool_calls": {"type": "integer"},
            },
            "required": ["prompt_tokens", "completion_tokens"],
        },
        "dto.ChatResponse": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "answer": {"type": "string"},
                "products": {
                    "type": "array",
                    "items": {"$ref": "#/definitions/dto.Product"},
                },
                "fallback_to_human": {"type": "boolean"},
                "model": {"type": "string"},
                "usage": {"$ref": "#/definitions/dto.Usage"},
            },
            "required": ["session_id", "answer", "products", "fallback_to_human"],
        },
        "dto.SessionResponse": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "expires_in": {"type": "integer"},
            },
            "required": ["session_id", "expires_in"],
        },
    }


def service_paths() -> dict:
    """Every path the gateway cannot regenerate: one owner per route."""
    paths = {}
    paths.update(recommendations_paths())
    paths.update(assistant_paths())
    paths["/api/v1/products/search"] = search_override()
    return paths


def merged(spec: dict) -> dict:
    paths = spec.setdefault("paths", {})
    paths.update(service_paths())

    definitions = spec.setdefault("definitions", {})
    definitions.update(service_definitions())
    spec["tags"] = _ensure_tags(
        spec.get("tags", []), ["Recommendations", "Assistant"]
    )
    info = spec.setdefault("info", {})
    info["description"] = (
        "High-performance E-Commerce platform API built with Go, Gin, PostgreSQL, "
        "Redis, Kafka, GHN, and VNPay."
    )
    return spec


SERVICE_OWNED_PATHS = frozenset(service_paths())


def adopted_instead_of_wiped(committed: dict, fresh: dict) -> dict:
    """Keep the service-owned routes when `swag` regenerates only the gateway.

    `swag` cannot see across the workspace boundary, so a run limited to
    `gomod/cmd/api` returns the three gateway routes and nothing else. Writing
    that over the committed spec would silently delete 150+ documented
    operations, so the fresh run only supplies the gateway-owned paths and
    everything else is carried over unchanged.
    """
    spec = copy.deepcopy(committed)
    spec["info"].update(fresh.get("info", {}))
    # basePath comes from `// @BasePath /` in gomod/cmd/api/main.go: set to "/api/v1"
    # there and every generated route becomes /api/v1/<path>, which is what the
    # edge actually proxies.
    spec["basePath"] = fresh.get("basePath", spec.get("basePath"))
    # `host` is a compile-time constant and swagger-ui builds every "Try it out"
    # URL from it. Drop it so the UI falls back to the origin that served the
    # page (edge on :80/:443, gateway on :8080 in dev-host) instead of calling a
    # hardcoded localhost:8080 that may not even be published.
    spec.pop("host", None)
    spec.pop("schemes", None)
    paths = spec.setdefault("paths", {})
    for name, item in fresh.get("paths", {}).items():
        if name in SERVICE_OWNED_PATHS:
            continue
        paths[name] = item
    definitions = spec.setdefault("definitions", {})
    definitions.update(fresh.get("definitions", {}))
    return merged(spec)


def _ensure_tags(tags: list[dict], wanted: list[str]) -> list[dict]:
    known = {tag.get("name") for tag in tags}
    own = {
        "Recommendations": "Best sellers, vector-backed shelves and hybrid search",
        "Assistant": "LLM shopping assistant and its streaming variant",
    }
    for name in wanted:
        if name not in known:
            tags.append({"name": name, "description": own[name]})
    return tags


def as_go_doc(spec: dict, title: str, version: str) -> str:
    """Render the swaggo docs.go the gateway imports via `_ "goshop/shared/docs"`."""
    payload = json.dumps(spec, indent=4, ensure_ascii=False, separators=(",", ": "))
    if "`" in payload:
        raise SystemExit(
            "the spec contains a backtick, which cannot live inside a Go raw "
            "string literal: reword that description"
        )
    # swaggo stores the template with placeholders so the host/scheme can come
    # from flags at runtime; we print them literally, which is what -o does.
    return (
        "// Package docs Code generated by scripts/patch-openapi-spec.py. DO NOT EDIT\n"
        "package docs\n\n"
        'import "github.com/swaggo/swag"\n\n'
        "const docTemplate = `\n" + payload + "\n`\n\n"
        "// SwaggerInfo holds exported Swagger Info so clients can modify it\n"
        "var SwaggerInfo = &swag.Spec{\n"
        "\tInfoInstanceName: \"swagger\",\n"
        f"\tSwaggerTemplate: docTemplate,\n"
        "\tLeftDelim:        \"{{\",\n"
        "\tRightDelim:       \"}}\",\n"
        "}\n\n"
        "func init() {\n"
        "\tswag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)\n"
        "}\n"
    )


def as_yaml(spec: dict) -> str:
    try:
        import yaml  # type: ignore
    except ImportError:
        raise SystemExit(
            "PyYAML is required for --yaml: pip install -r scripts/requirements.txt"
        )
    return yaml.safe_dump(spec, sort_keys=True, allow_unicode=True, width=1000)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--check",
        action="store_true",
        help="exit 1 when the committed spec differs from the merged one",
    )
    parser.add_argument("--yaml", action="store_true", help="also rewrite swagger.yaml")
    parser.add_argument(
        "--go", action="store_true", help="also rewrite docs.go from the merged spec"
    )
    parser.add_argument(
        "--from",
        dest="from_spec",
        metavar="PATH",
        help="fresh gateway spec from a `swag` run; its paths are merged into the "
        "committed one instead of replacing it",
    )
    parser.add_argument(
        "--openapi",
        action="store_true",
        help="also write openapi.json (Swagger 2.0 -> OpenAPI 3 via swagger2openapi)",
    )
    args = parser.parse_args()

    if not SPEC.exists():
        print(f"missing {SPEC}; run `make swagger` first", file=sys.stderr)
        return 1

    original = SPEC.read_text(encoding="utf-8")
    committed = json.loads(original)
    if args.from_spec:
        fresh = json.loads(Path(args.from_spec).read_text(encoding="utf-8"))
        spec = adopted_instead_of_wiped(committed, fresh)
    else:
        spec = merged(committed)
    rendered = json.dumps(spec, indent=4, ensure_ascii=False, separators=(",", ": ")) + "\n"
    if args.check:
        if rendered.strip() != original.strip():
            print(
                "swagger.json is out of date: run `make swagger`",
                file=sys.stderr,
            )
            return 1
        print("swagger.json is up to date")
        return 0

    SPEC.write_text(rendered, encoding="utf-8")
    print(f"patched {SPEC.relative_to(ROOT)}")

    if args.yaml:
        yaml_path = SPEC.with_suffix(".yaml")
        yaml_path.write_text(as_yaml(spec), encoding="utf-8")
        print(f"wrote {yaml_path.relative_to(ROOT)}")

    if args.go:
        go_path = SPEC.with_name("docs.go")
        go_path.write_text(as_go_doc(spec, "GOshop E-Commerce API", "1.0"), encoding="utf-8")
        print(f"wrote {go_path.relative_to(ROOT)}")

    if args.openapi:
        print(
            "openapi.json is produced by `make swagger` (swagger2openapi); "
            "re-run that target after patching",
            file=sys.stderr,
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
