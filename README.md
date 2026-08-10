# Arqut Edge - Community Edition

Open-source edge proxy service with embedded web UI.

## Quick Start

```bash
# Install UI dependencies (first time only)
make install-ui

# Build everything (UI + Go binary)
make build

# Run the server
make run
```

The server will start on **http://localhost:3030** with the embedded UI.

## Development

### UI Development

```bash
# Start UI dev server with hot reload (proxies API to :3030)
make dev-ui

# In another terminal, run the Go backend
ARQUT_API_KEY=test SERVER_ADDR=:3030 DB_PATH=/tmp/edge.db go run ./cmd/arqut-edge-ce
```

UI will be available at **http://localhost:9000** with live reload.

### Build Commands

```bash
make build       # Build UI + Go binary
make build-ui    # Build UI only
make build-go    # Build Go binary only
make clean       # Clean build artifacts
make test        # Run tests
make help        # Show all targets
```

## Manual Build

```bash
# Build UI
cd ui && npm run build && cd ..

# Build Go binary with embedded UI
go build -o bin/arqut-edge-ce-app ./cmd/arqut-edge-ce

# Run
ARQUT_API_KEY=test SERVER_ADDR=:3030 DB_PATH=/tmp/edge.db ./bin/arqut-edge-ce-app
```

## Configuration

Environment variables:
- `ARQUT_API_KEY` - API authentication key (required)
- `SERVER_ADDR` - Server listen address (default: `:3030`)
- `DB_PATH` - Database file path (default: `./data/edge.db`)
- `CLOUD_URL` - Cloud server URL for WebRTC signaling (optional)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Contributions are accepted under the Developer Certificate of Origin. Sign off
your commits with `git commit -s`; CI checks this on every pull request.

To report a security vulnerability, see [SECURITY.md](SECURITY.md) — please do
not open a public issue.

## License

Licensed under the Apache License, Version 2.0 — see [LICENSE](LICENSE).

```
SPDX-License-Identifier: Apache-2.0
```

Copyright 2024-2026 semilimes AG.

Third-party components are attributed in [NOTICE](NOTICE), with the full
component list and license texts in
[THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES). This covers both the Go modules
compiled into the binary and the JavaScript packages bundled into the embedded
management UI. No GPL, LGPL, or AGPL code is distributed with this project.
