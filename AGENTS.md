# AGENTS.md — Terraform/OpenTofu Provider for PurelyMail

Guidance for AI agents (and humans) working in this repository. Follow these conventions unless a task explicitly says otherwise.

## Project Overview

This repository implements an **OpenTofu/Terraform provider** for [PurelyMail](https://purelymail.com), a hosted email service. The provider lets users manage PurelyMail resources (domains, users, DNS records, routing rules, etc.) declaratively via `tofu`/`terraform`.

- **Language:** Go
- **Framework:** [terraform-plugin-framework](https://developer.hashicorp.com/terraform/plugin/framework) (the current recommended framework — do NOT use legacy SDKv2)
- **Compatibility:** Targets OpenTofu 1.x and Terraform 1.x via the plugin protocol (protocol version 6+)
- **Provider address:** reserved namespace placeholder (e.g. `registry.terraform.io/<org>/purelymail`) — update once the registry namespace is known

## First Tasks for New Agents

1. **Build our own API client.** No suitable third-party Go client exists, so this repo maintains its own client under `internal/purelymail/`, written against the official PurelyMail API docs (Bearer-token auth against `https://api.purelymail.com`).
   - Start **minimal**: auth + the endpoint(s) needed by the resource currently being built; grow the client incrementally as resources are added.
   - Keep it thin: typed request/response structs, error handling, and pagination only where the API requires it. No convenience layers beyond what the provider needs.
2. Scaffold with `terraform-plugin-scaffold-framework` layout conventions (see Repository Layout below).
3. Confirm the PurelyMail API surface (endpoints, auth) from current official docs before modeling any resource.

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

Implement, in roughly this priority order:

- `purelymail_domain` — domain ownership/verification
- `purelymail_user` — mailbox users, passwords, forwarding
- `purelymail_domain_dns` / DNS records — required records for a domain
- `purelymail_routing_rule` — routing/catch-all rules

Each resource needs a matching data source only if useful for lookups (e.g. domain lookup); do not add data sources speculatively.

## Provider Configuration

- API authentication token (Bearer) via provider schema attribute, falling back to env var `PURELYMAIL_API_TOKEN`.
- Base URL override attribute for testing.
- Never log the token; redact in diagnostics and debug output.

## Coding Conventions

- Standard Go project layout; package `main` only in `main.go`.
- Resources live in `internal/provider`, one file per resource, named `<name>_resource.go`; data sources `<name>_data_source.go`.
- Use framework types (`types.String`, etc.) and implement all standard CRUD methods + `ImportState`.
- API client calls go through a single client type constructed in `Configure`; resources never build HTTP requests directly.
- Map API errors to actionable diagnostics; include the API error body text.
- Timeouts: support `timeouts` block on resources with sane defaults (1–2 min).

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
