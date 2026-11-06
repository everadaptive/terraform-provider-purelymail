# AGENTS.md — Terraform/OpenTofu Provider for PurelyMail

Guidance for AI agents (and humans) working in this repository. Follow these conventions unless a task explicitly says otherwise.

## Project Overview

This repository implements an **OpenTofu/Terraform provider** for [PurelyMail](https://purelymail.com), a hosted email service. The provider lets users manage PurelyMail resources (domains, users, DNS records, routing rules, etc.) declaratively via `tofu`/`terraform`.

- **Language:** Go
- **Framework:** [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework) (the current recommended framework — do NOT use legacy SDKv2)
- **Compatibility:** Targets OpenTofu 1.x and Terraform 1.x via the plugin protocol (protocol version 6+)
- **Provider address:** `registry.opentofu.org/everadaptive/purelymail` (Go module: `github.com/everadaptive/terraform-provider-purelymail`)

## API Facts (confirmed against the official spec)

- **Spec:** https://news.purelymail.com/api/index.html (Swagger UI; the OpenAPI JSON is served at `https://news.purelymail.com/api/swagger-spec.js` as `window.swaggerSpec=…`)
- **Auth:** API key header `Purelymail-Api-Token` — **not** Bearer
- **Base URL:** `https://purelymail.com`, all endpoints are `POST` under `/api/v0/…`
- **Responses:** wrapped as `{"type": "success", "result": <T>}`; errors return `{"code": …, "message": …}`

## First Tasks for New Agents

1. **Build our own API client.** No suitable third-party Go client exists, so this repo maintains its own client under `internal/purelymail/`, written against the official API spec (see API Facts above).
   - Start **minimal**: auth + the endpoint(s) needed by the resource currently being built; grow the client incrementally as resources are added.
   - Keep it thin: typed request/response structs, error handling, and pagination only where the API requires it. No convenience layers beyond what the provider needs.
2. Confirm the PurelyMail API surface from the official spec before modeling any resource.
3. **Docs and tests ship with every change** — see "Docs & Tests As You Go" below; a resource change is not done until its docs and tests are updated in the same commit series.

## Repository Layout

```
main.go                  # provider entrypoint (also serves plugin-mode for SDK debugging)
internal/provider/       # provider schema, resource & data source registration
internal/provider/…      # one file per resource/datasource (e.g. user_resource.go)
internal/purelymail/     # our own API client (auth, typed request/response structs)
examples/                # example .tf configurations per resource
docs/                    # provider & resource documentation (registry-style)
```

## Resources (target coverage)

- [x] `purelymail_domain` — domain resource (CRUD + import) and data source
- [x] `purelymail_user` — user resource (CRUD + import)
- [x] `purelymail_dns_records` — data source emitting the standard record set per domain
- [x] `purelymail_routing_rule` — routing/catch-all rules
- [x] `purelymail_app_password`, `purelymail_password_reset` — secondary resources
- [x] `purelymail_sieve_script` — Sieve filter scripts via ManageSieve (experimental)

Each resource needs a matching data source only if useful for lookups (e.g. domain lookup); do not add data sources speculatively.

## Live Test Harness

The sibling repo `../cloudflare-dns` (github.com:everadaptive/cloudflare-dns) doubles as the live integration test project:

- `~/.tofurc` has `dev_overrides` pointing `everadaptive/purelymail` at `<this repo>/bin`, so `tofu` there runs the locally built binary. Rebuild with `go build -o bin/terraform-provider-purelymail .` after any code change.
- Existing domains and users are imported there (`terraform.tfstate` holds the imports); a clean `tofu plan` (no changes) is the acceptance check for provider changes.
- Keep commits flowing in **both** repos as work progresses.

## Provider Configuration

- API authentication token via the `Purelymail-Api-Token` header, provided through the provider schema attribute, falling back to env var `PURELYMAIL_API_TOKEN`.
- Base URL override attribute for testing.
- Never log the token; redact in diagnostics and debug output.

## Coding Conventions

- Standard Go project layout; package `main` only in `main.go`.
- Resources live in `internal/provider`, one file per resource, named `<name>_resource.go`; data sources `<name>_data_source.go`.
- Use framework types (`types.String`, etc.) and implement all standard CRUD methods + `ImportState`.
- API client calls go through a single client type constructed in `Configure`; resources never build HTTP requests directly.
- Map API errors to actionable diagnostics; include the API error body text.
- Timeouts: support `timeouts` block on resources with sane defaults (1–2 min).

## Docs & Tests As You Go

Every resource or client change ships with its own tests and docs — do not defer them to a later task:

- **Tests:** each new client endpoint gets unit tests (httptest) covering auth header, envelope decoding, and error mapping, before or alongside the resource that uses it (`go test ./...`).
- **Examples:** every new resource/data source gets an example under `examples/<category>/<name>/` in the same change.
- **Docs:** every new or schema-changed resource/data source updates `docs/` in the same change (registry-style pages: argument reference, import syntax, live example).
- A task is not done until `go build ./... && go test ./...` passes and the docs/example for the change exist.

## Testing

- Unit tests for client mapping logic (`go test ./...`).
- Acceptance tests with `tf_acc_tests`-style env-var gating (`TF_ACC=1`), skipped by default. They require a real PurelyMail API token: set `PURELYMAIL_API_TOKEN`.
- Every resource needs at least basic CRUD + import acceptance coverage; tests must clean up after themselves.
- Run `go build ./... && go test ./...` before declaring any task done.

## Tooling

- `golangci-lint run` — must pass.
- `go mod tidy` after dependency changes.
- Docs under `docs/` must be updated whenever a resource schema changes (generated templates acceptable).

## Agent Workflow Rules

- Keep changes focused: one resource or concern per change; do not refactor unrelated code.
- Do not commit secrets or real API tokens — tests use env vars only.
- When API behavior is undocumented or ambiguous, note the assumption in code comments and raise it to the user rather than guessing silently.
- Schema changes that break state must include guidance (state upgrader or documented migration step).
