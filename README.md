# hoardCTI/schemas

The `schemas` repo is the single source of truth for the Hoard Schema — the standardized transport format used by every ingest feed in Hoard CTI to pass data into the central CTI parser.

## Why this repo exists

Hoard CTI is a decentralized aggregation of open source threat intelligence — one repo per source (CVE, IOC, threat actors, hidden service indexes, etc.). For the central process to consume data from any of these independently maintained repos, every ingest repo needs to output data in a single, predictable shape.

This repo defines that shape, so that:

- Any ingest repo can be validated independently, without needing the central process
- New sources can be added without changing the central parser
- Contributors building a new ingest repo have a clear, versioned contract to build against

## Schema validation

Every `*.schema.json` file in the repo is validated by a Go test suite in [`tests/`](tests), driven by the [schema-tests workflow](.github/workflows/schema-tests.yml). On pull requests, only the schema files changed in the PR are tested (the whole suite re-runs if the tests themselves change, and the check passes immediately if no schema changed); pushes to `main` always validate everything. The `Validate schemas` check is required for merging. Two checks run per schema:

1. **Syntax & style** — well-formed UTF-8 JSON with no duplicate keys, consistent whitespace, the required top-level keywords (`$schema`, `$id`, `title`, `description`, `type`), and an `$id` that matches the file's published URL.
2. **Meta-schema validation** — the schema is validated against the meta-schema declared in its `$schema` keyword (normally JSON Schema draft 2020-12), and the check is repeated recursively up the meta-schema chain (up to 5 levels, with each remote document fetched once and cached).

Run locally:

```sh
cd tests
go test ./...        # full suite
go test -short ./... # style checks only (no network)
```