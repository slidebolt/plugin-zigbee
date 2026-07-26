# Git Workflow for plugin-zigbee

This repository contains the Slidebolt Zigbee plugin. It mirrors the standalone `zigbee-host` API into Slidebolt and produces a standalone Go binary.

## Dependencies

- **Internal:** `sb-contract`, `sb-domain`, `sb-messenger-sdk`, `sb-runtime`, and `sb-storage-sdk`.
- **External service:** `zigbee-host`, configured through `ZIGBEE_HOST_URL` and `ZIGBEE_HOST_TOKEN`.

## Build Process

- **Type:** Go application (plugin).
- **Artifact:** `plugin-zigbee`.
- **Build:** `go build -o plugin-zigbee ./cmd/plugin-zigbee`
- **Validation:** `go test ./...` and `go vet ./...`

## Publishing

1. Ensure internal dependencies and `zigbee-host` API changes are published.
2. Run `go mod tidy`, `go test ./...`, and `go vet ./...`.
3. Build the binary successfully.
4. Commit and push changes to `main`.
5. Create and push the next semantic version tag (`vX.Y.Z`).

Tags trigger the release workflow, which builds Linux AMD64 and ARM64 binaries.
