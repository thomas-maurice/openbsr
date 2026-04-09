# OpenBSR

A self-hosted, open-source alternative to the [Buf Schema Registry (BSR)](https://buf.build). Push, pull, and manage Protobuf modules on your own infrastructure — fully compatible with the `buf` CLI.

## Features

- **`buf push` / `buf dep update`** — Full ConnectRPC API compatibility (`buf.registry.module.v1`)
- **Organizations** — Create orgs, manage members with admin/member roles
- **Access control** — Public and private modules, Casbin-based RBAC authorization
- **Proto reflection** — `FileDescriptorSetService` compiles stored protos via `bufbuild/protocompile`
- **Web UI** — Vue 3 SPA with module browser, proto syntax highlighting, token management
- **Multiple DB backends** — SQLite (default), PostgreSQL, MongoDB
- **Pluggable blob storage** — Local filesystem, S3-compatible (AWS, Garage, R2), or database
- **Single binary** — All assets embedded via `embed.FS`, no external runtime dependencies

## Quick Start

```bash
# Build (requires Go 1.22+ and CGO for SQLite)
make build

# Run — starts on :8080 with SQLite + local storage, zero dependencies
./bsr
```

Or with Docker:

```bash
docker compose up -d
# → http://localhost:18080
# Admin: admin / changeme
```

## Deployment Examples

Pre-built Docker Compose files are in the `deploy/` directory:

### SQLite (simplest — no external services)

```bash
docker compose up -d
```

Everything in one container. Data persists in a Docker volume. Good for development and small teams.

### MongoDB

```bash
docker compose -f deploy/docker-compose.mongo.yml up -d
```

Starts MongoDB 7 alongside the app. Module metadata in Mongo, blobs on local filesystem.

### SQLite + Garage (S3-compatible storage)

[Garage](https://garagehq.deuxfleurs.fr/) is a lightweight, self-hosted S3-compatible storage backend. Good for production when you want blob storage separate from the database.

```bash
# Start the stack
docker compose -f deploy/docker-compose.garage.yml up -d

# Wait ~5 seconds, then bootstrap Garage (creates bucket + API key)
bash deploy/garage-setup.sh
```

The setup script creates the S3 bucket, generates an API key, and restarts the app with the correct credentials. You only need to run it once.

## Database Configuration

| Backend | `DB_DRIVER` | Required config | Notes |
|---------|------------|-----------------|-------|
| **SQLite** | `sqlite` (default) | `SQLITE_PATH` (default: `./data/bsr.db`) | Zero dependencies. Single file. |
| **PostgreSQL** | `postgres` | `POSTGRES_DSN` (e.g. `postgres://user:pass@host:5432/bsr?sslmode=disable`) | Production-ready. |
| **MongoDB** | `mongo` | `MONGO_URI` (default: `mongodb://localhost:27017/bsr`) | Legacy. |

## Storage Configuration

Module content (zipped `.proto` files) is stored separately from metadata. Choose a storage backend:

| Backend | `STORAGE_DRIVER` | Required config | Notes |
|---------|-----------------|-----------------|-------|
| **Local filesystem** | `local` (default) | `STORAGE_PATH` (default: `./data/modules`) | Simplest. Single machine. |
| **S3-compatible** | `s3` | `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | Any S3 API: AWS, Garage, Cloudflare R2, Scaleway. |
| **Database** | `db` | (uses the control plane DB) | Blobs stored in SQL alongside metadata. Requires `DB_DRIVER=sqlite` or `postgres`. |

### S3 storage variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_ENDPOINT` | — | S3 endpoint URL (e.g. `http://garage:3900`, `https://s3.amazonaws.com`) |
| `S3_REGION` | `us-east-1` | S3 region (use `garage` for Garage, `auto` for R2) |
| `S3_BUCKET` | `openbsr` | Bucket name |
| `S3_PREFIX` | — | Optional key prefix within the bucket |
| `S3_ACCESS_KEY` | — | Access key ID |
| `S3_SECRET_KEY` | — | Secret access key |

### Mixing backends

Database and storage backends are independent. You can mix and match:

| Metadata | Blobs | Good for |
|----------|-------|----------|
| SQLite | Local | Development, single instance |
| SQLite | S3 (Garage) | Small team, durable blob storage |
| PostgreSQL | S3 (AWS) | Production |
| PostgreSQL | DB | Simplest production (everything in one DB) |
| MongoDB | Local | Legacy deployments |

## Other Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `BSR_PORT` | `8080` | HTTP server port |
| `ADMIN_USERNAME` | — | Auto-created admin username on startup |
| `ADMIN_PASSWORD` | — | Auto-created admin password on startup |
| `OPEN_REGISTRATION` | `false` | Allow public user registration |
| `TLS_CERT` | — | TLS certificate file path (enables HTTPS) |
| `TLS_KEY` | — | TLS private key file path |

## Using with the `buf` CLI

### 1. Get an API token

Register and log in via the web UI, then create an API token under **Tokens**.

### 2. Configure `buf.yaml`

```yaml
version: v2
modules:
  - path: proto
    name: your-registry.example.com/youruser/yourmodule
```

### 3. Push

```bash
export BUF_TOKEN=<your-api-token>
buf push
```

> **Self-signed TLS:** The `buf` CLI requires trusted TLS. For local dev, use `mkcert -install` to add a local CA, or push via the ConnectRPC API directly with `curl -k` (see the **Docs** page in the UI for examples).

### 4. Use as a dependency

```yaml
deps:
  - your-registry.example.com/otheruser/theirmodule
```

```bash
export BUF_TOKEN=<your-api-token>
buf dep update
```

## API

### ConnectRPC (used by `buf` CLI)

| Service | Package | Endpoints |
|---------|---------|-----------|
| `AuthnService` | `buf.alpha.registry.v1alpha1` | `GetCurrentUser` |
| `OwnerService` | `buf.registry.owner.v1` | `GetOwners` |
| `OrganizationService` | `buf.registry.owner.v1` | `GetOrganizations`, `CreateOrganizations` |
| `UserService` | `buf.registry.owner.v1` | `GetUsers`, `GetCurrentUser`, `CreateUsers` |
| `ModuleService` | `buf.registry.module.v1` | `GetModules`, `ListModules`, `CreateModules` |
| `UploadService` | `buf.registry.module.v1` | `Upload` |
| `DownloadService` | `buf.registry.module.v1` | `Download` |
| `CommitService` | `buf.registry.module.v1` | `GetCommits`, `ListCommits` |
| `LabelService` | `buf.registry.module.v1` | `GetLabels`, `ListLabels`, `CreateOrUpdateLabels` |
| `FileDescriptorSetService` | `buf.registry.module.v1` | `GetFileDescriptorSet` |

### REST API (`/api/v1/`)

```
POST   /api/v1/auth/register
POST   /api/v1/auth/login
GET    /api/v1/auth/me
GET    /api/v1/auth/tokens
POST   /api/v1/auth/tokens
DELETE /api/v1/auth/tokens/:id

GET    /api/v1/users/:username
GET    /api/v1/orgs/:name
POST   /api/v1/orgs
POST   /api/v1/orgs/:name/members
DELETE /api/v1/orgs/:name/members/:username

GET    /api/v1/modules
GET    /api/v1/modules/:owner/:repo
POST   /api/v1/modules
GET    /api/v1/modules/:owner/:repo/commits
GET    /api/v1/modules/:owner/:repo/commits/:id
GET    /api/v1/modules/:owner/:repo/commits/:id/files
GET    /api/v1/modules/:owner/:repo/commits/:id/file?path=...
GET    /api/v1/modules/:owner/:repo/labels
```

## Authorization

Authorization is centralized via [Casbin](https://casbin.org) with an RBAC model:

| Role | Read module | Push | Admin (labels, delete) | Create modules | Manage org members |
|------|:-----------:|:----:|:----------------------:|:--------------:|:------------------:|
| Owner | yes | yes | yes | yes | — |
| Org admin | yes | yes | yes | yes | yes |
| Org member | yes | yes | — | — | — |
| Public | public only | — | — | — | — |

## Development

```bash
# Run all tests (requires CGO for SQLite)
CGO_ENABLED=1 go test ./...

# Run E2E tests (in-process, no Docker, ~1.5s)
CGO_ENABLED=1 go test -v ./test/

# Run S3 E2E test (requires Docker for Garage)
OPENBSR_TEST_S3=1 CGO_ENABLED=1 go test -v -run TestS3Storage ./test/

# Vendor frontend assets
make vendor-frontend
```

See [CLAUDE.md](CLAUDE.md) for the full architecture reference and development guide.

## License

[MIT](LICENSE) — Copyright (c) 2026 Thomas Maurice
