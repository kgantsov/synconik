# synconik [![codecov](https://codecov.io/gh/kgantsov/synconik/graph/badge.svg?token=FP40VBWNOW)](https://codecov.io/gh/kgantsov/synconik)

synconik is a Go daemon that watches a local directory and mirrors it into
[Iconik](https://www.iconik.io/), a media asset management platform. Directories
become Iconik **collections** and files become Iconik **assets**. A BadgerDB store
records what has already been synced so nothing is uploaded twice.

Beyond one-way uploads, synconik keeps the local folder and Iconik in sync in both
directions: it pushes new files up, pulls originals back down when they are requested
onto the local storage, and removes local files when their file set is deleted in Iconik.

## Features

- Periodic file-system scanning that mirrors the directory tree into Iconik
  (directories → collections, files → assets)
- Concurrent uploads via a configurable worker pool
- Pluggable cloud storage backends for the actual byte upload: **GCS**, **S3**, and **B2**
- Local ("FILE" method) storage mirroring — pull originals back to disk when Iconik
  requests a transfer onto the local storage
- Deletion sync — remove local files whose file set was deleted in Iconik
- Persistent sync state using BadgerDB (keyed by path relative to the scan directory)
- Configurable logging with zerolog
- Graceful shutdown on SIGINT/SIGTERM
- Command-line interface using Cobra, configured with Viper (file, flags, and env vars)

## Prerequisites

- Go 1.x or higher
- Access to an Iconik instance with an App ID, token, and storage IDs

## Installation

### Quick install (recommended)

The installer detects your OS/architecture, downloads the matching release binary
to `/usr/local/bin`, and sets it up as a managed service — a **systemd** service on
Linux or a **launchd** daemon on macOS — with log rotation capped so it can't fill
the disk:

```bash
curl -fsSL https://raw.githubusercontent.com/kgantsov/synconik/main/install.sh | sh
```

It writes a config template (`/etc/synconik/config.yaml` on Linux,
`/usr/local/etc/synconik/config.yaml` on macOS) that you fill in before starting the
service. Follow the on-screen instructions the script prints when it finishes.

Optional environment overrides:

```bash
# pin a version, change the install dir, or raise the log cap (MB)
VERSION=v1.2.3 INSTALL_DIR=/usr/local/bin LOG_MAX_MB=500 \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/kgantsov/synconik/main/install.sh)"

# install the binary only, skip the service setup
NO_SERVICE=1 sh -c "$(curl -fsSL https://raw.githubusercontent.com/kgantsov/synconik/main/install.sh)"
```

Managing the service:

```bash
# Linux (systemd)
sudo systemctl enable --now synconik                   # start at boot + now
sudo systemctl status synconik
sudo journalctl --namespace synconik -u synconik -f    # follow logs

# macOS (launchd)
sudo launchctl load -w /Library/LaunchDaemons/io.iconik.synconik.plist
tail -f /usr/local/var/log/synconik.log
```

### From source

1. Clone the repository:
```bash
git clone https://github.com/kgantsov/synconik.git
cd synconik
```

2. Install dependencies:
```bash
go mod download
```

3. Build the binary:
```bash
go build -o synconik main.go
```

## Configuration

Configuration is loaded by Viper from `config.yaml` in the working directory
(override with `--config`). Every value can also be supplied as a CLI flag or an
environment variable.

The following parameters are **required** (enforced at startup):
`scanner.dir`, `iconik.app_id`, `iconik.token`, `iconik.storage_id`,
`iconik.local_storage_id`, and `store.data_dir`.

Example `config.yaml`:
```yaml
iconik:
  url: "https://app.iconik.io"
  app_id: "your-app-id"
  token: "your-token"
  # Cloud storage (GCS/S3/B2) that assets are uploaded to
  storage_id: "your-storage-id"
  # Local ("FILE" method) storage that mirrors scanner.dir on this machine
  local_storage_id: "your-local-storage-id"
  # Optional: nest every new collection/asset under an existing collection.
  # collection_id: "your-root-collection-id"

store:
  data_dir: "db"          # directory for the BadgerDB sync state

scanner:
  dir: "data"             # directory to watch and mirror into Iconik
  interval: 60            # seconds between scans / sync polls

uploader:
  workers: 5              # number of concurrent upload workers
  stability_window: 30    # seconds to wait for a file to stop changing before uploading

log:
  level: "info"           # trace | debug | info | warn | error
```

> The YAML logging key is `log:` but it maps to `logging.level` internally.

## Usage

Run the application:
```bash
./synconik                       # uses ./config.yaml
./synconik --config /path/to/config.yaml
```

Or run directly from source:
```bash
go run main.go
```

While running, the daemon will:
1. Initialize the BadgerDB store.
2. Start the upload worker pool.
3. Scan `scanner.dir` every `scanner.interval` seconds, creating collections for new
   directories and uploading new files as assets to the configured cloud storage.
4. Poll Iconik on the same interval to pull requested originals back to disk (restore)
   and to remove local files whose file set was deleted in Iconik (deletion sync).
5. Shut down gracefully on SIGINT/SIGTERM, draining in-flight jobs first.

## Testing

```bash
go test ./...                                     # run all tests
go test ./... -cover -coverprofile coverage.out   # with coverage (matches CI)
go test ./internal/storage/ -run TestName -v      # run a single test
```

## Project Structure

```
.
├── main.go              # Application entry point (wires the pipeline together)
├── internal/
│   ├── config/          # Viper/Cobra configuration management
│   ├── iconik/          # Iconik REST API client
│   ├── scanner/         # File-system scanner (produces upload jobs)
│   ├── storage/         # Cloud byte-upload backends (GCS, S3, B2)
│   ├── store/           # BadgerDB sync-state persistence
│   ├── uploader/        # Worker-pool upload dispatcher
│   ├── entity/          # Core domain entities
│   └── usecase/         # Business logic (asset, collection, restore, deletion)
├── config.yaml          # Runtime configuration
└── db/                  # BadgerDB data directory
```
