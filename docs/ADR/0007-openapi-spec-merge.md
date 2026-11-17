# ADR 0007 — The OpenAPI spec is merged, not generated in one pass

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** the gateway spec and `@goshop/api-client`
- **Code:** `scripts/patch-openapi-spec.py`, `Makefile` (`swagger`, `swagger-check`)

## Context

`swag init` needs to parse the whole package graph to emit one document. The backend is a Go
workspace with **one module per service**, and `swag` cannot parse across module boundaries.
Pointed at the gateway alone it sees 3 paths and would silently delete the other 137. Given
more than one module it dies on `cannot find type definition: utils.APIResponse`.

The spec is not decorative: `frontend/packages/api-client` is generated from it, so a silently
truncated document breaks both apps at build time with errors that point at generated code.

## Decision

`make swagger` splits the job:

1. `swag` regenerates **only the gateway's own routes** into a scratch directory.
2. `scripts/patch-openapi-spec.py` merges them into the committed spec and re-asserts the
   routes that live in service modules (recommendations, assistant, hybrid search) from
   definitions kept inside the script.
3. The script also normalises two fields that would otherwise break "Try it out":
   - drops `host`/`schemes`, so Swagger UI calls whatever origin served the page (edge on
     `:80`/`:443`, gateway on `:8080` in dev-host) instead of a hardcoded `localhost:8080`;
   - keeps `basePath: /api/v1`, because the edge only proxies `/api/v1/*` — without it every
     documented path builds a URL that falls through to the storefront's Next.js 404.
4. `swagger2openapi` produces the OpenAPI 3.0 file the TypeScript client is generated from.

`make swagger-check` fails CI when the committed spec drifted from the code.

## Consequences

**Positive**

- One spec, one generated client, both apps type-checked against the same surface.
- Drift between the script and the committed spec is a build failure, not a support ticket.

**Trade-offs**

- A route annotated inside `gomod/services/*` **does not appear** until it is also added to
  `patch-openapi-spec.py`. `make swagger-check` catches drift from the script, not a route the
  script has never heard of.
- Two places describe some routes: the handler annotation and the script's declaration.

## Rejected alternatives

- **Run `swag` per module and concatenate** — path duplicates (every service declares
  `/health`) and conflicting `definitions` names; the merge would need a disambiguation rule
  that is harder to review than an explicit list.
- **Hand-write the spec** — guaranteed to drift from the handlers within a sprint.
- **Drop to a single Go module to satisfy `swag`** — the module boundary is what enforces
  ownership; a tool should not dictate the architecture.
- **Generate the client from each service's own spec** — two clients, two base URLs, and the
  browser would need direct access to fourteen services.
