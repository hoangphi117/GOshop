# GOshop Development Conventions

## Language & Localization
- **UI & Text Convention:** All user interface text, page titles, navigation labels, form labels, placeholders, validation error messages, alerts, and notification toasts must be written in **English**.
- Do not hardcode Vietnamese strings in UI components, notifications, or toasts unless specifically requested by the user.

## Architecture Guidelines
- **Monorepo Structure:** Turborepo + pnpm workspaces.
  - `frontend/packages/api-client`: OpenAPI type-safe client generator (`@goshop/api-client`).
  - `frontend/apps/dashboard`: Admin Dashboard (Vite + React 19 + TanStack Router + TanStack Query + Tailwind CSS).
  - `frontend/apps/storefront`: Storefront (Next.js App Router).
- **TypeScript:** Strict type checking. No `any` casting. Form schemas must match input and output types for seamless React Hook Form integration.
- **Authentication:** JWT Access Token + Refresh Token with automatic silent refresh & retry mechanism in `@goshop/api-client`.

## Output Style (ADHD-Friendly)
The reader has ADHD. Shape every response so it can be acted on:
1. **Lead with the action**: Command, path, or code snippet first. Prose after, if at all.
2. **Number multi-step tasks**: One bounded action per step.
3. **End with one next action**: A concrete step doable in under two minutes.
4. **Suppress tangents**: Finish the current issue before bringing up another.
5. **Restate progress each turn**: E.g., "Step 3 of 5 done".
6. **Specific time estimates**: Use concrete units (minutes/hours), never vague terms.
7. **Make completed work visible**: Show what now works and how to verify it.
8. **Direct error reporting**: State file location, cause, and fix without fluff.
9. **Cap lists to 5 items**: Group related items and prioritize the most important.
10. **No fluff**: No preambles ("Sure!", "Let me..."), recaps, or closing pleasantries.

Exceptions: Explain fully when explicitly asked to explain. Confirm before destructive actions.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **GOshop** (3957 symbols, 9753 relationships, 322 execution flows).

> Index stale? Run `node .gitnexus/run.cjs analyze --index-only` from the project root — it auto-selects an available runner. No `.gitnexus/run.cjs` yet? Bootstrap with `npx`, `bunx`, or `pnpm dlx` — e.g. `bunx gitnexus@latest analyze` (npm 11 npx crash; #1939).

## Always Do

- **MUST run impact before editing.** Use `impact({target: "symbolName", direction: "upstream"})` or `node .gitnexus/run.cjs impact "symbolName" --direction upstream --repo .`; report callers, processes, and risk. Never substitute grep for graph analysis.
- **MUST analyze graph changes before committing.** Use `detect_changes({scope: "all"})` (MCP) or `node .gitnexus/run.cjs detect-changes --scope all --repo .` (CLI fallback). `partial: true` or `truncated: true` is not a clean check — a zero means unseen, not unaffected; re-run it. For regression review: `detect_changes({scope: "compare", base_ref: "main"})` or `node .gitnexus/run.cjs detect-changes --scope compare --base-ref "main" --repo .`.
- MUST warn on HIGH/CRITICAL `risk` pre-edit; never use `riskSharedAxes` to waive a HIGH/CRITICAL `risk` warning. Compare File/symbol: MCP File omits axes; Graph-RAG expands File.
- **MUST treat `risk: UNKNOWN` as unresolved, not as low.** An empty caller set is not evidence the symbol is unused — it can also mean the callers are not resolvable by the index (plain-object property access, dynamic dispatch, cross-language calls). `impact` pairs `UNKNOWN` with a `riskNote` saying so. Confirm with a text search before treating the symbol as safe to change or delete; do not proceed on the strength of a zero.
- **MUST use `query({search_query: "concept"})` for concepts/flows, `context({name: "symbolName"})` for a named symbol, or `impact` for blast radius, on read-only callers, dependencies, imports, or execution flow.** Graph first; text search only for empty/`UNKNOWN`/literals.
- For security review, `explain({target: "fileOrSymbol"})` lists taint findings (source→sink flows; needs `analyze --pdg`).

## Never Do

- NEVER edit a function, class, or method before MCP/CLI impact analysis.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis, and never read `UNKNOWN` as an all-clear — it means the walk could not answer, which is the one verdict that requires confirming by other means.
- NEVER rename symbols with find-and-replace — use `rename` which understands the call graph.
- NEVER commit before MCP/CLI graph change analysis.

## Resources

| Resource | Use for |
| --- | --- |
| `gitnexus://repo/GOshop/context` | Codebase overview, check index freshness |
| `gitnexus://repo/GOshop/clusters` | All functional areas |
| `gitnexus://repo/GOshop/processes` | All execution flows |
| `gitnexus://repo/GOshop/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
| --- | --- |
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
