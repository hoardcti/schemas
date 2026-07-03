# hoardCTI/schemas

The `schemas` repo is the single source of truth for the Hoard Schema — the standardized transport format used by every ingest feed in Hoard CTI to pass data into the central CTI parser.

## Why this repo exists

Hoard CTI is a decentralized aggregation of open source threat intelligence — one repo per source (CVE, IOC, threat actors, hidden service indexes, etc.). For the central process to consume data from any of these independently maintained repos, every ingest repo needs to output data in a single, predictable shape.

This repo defines that shape, so that:

- Any ingest repo can be validated independently, without needing the central process
- New sources can be added without changing the central parser
- Contributors building a new ingest repo have a clear, versioned contract to build against