# synconik [![codecov](https://codecov.io/gh/kgantsov/synconik/graph/badge.svg?token=FP40VBWNOW)](https://codecov.io/gh/kgantsov/synconik)

synconik is a Go-based file synchronization tool designed to monitor and upload files to Iconik, a media management platform. It provides efficient file scanning and uploading capabilities with persistent storage using BadgerDB.

## Features

- Real-time file system monitoring
- Efficient file upload to Iconik
- Persistent storage using BadgerDB
- Configurable logging
- Graceful shutdown handling
- Command-line interface using Cobra

## Prerequisites

- Go 1.x or higher
- Access to an Iconik instance with appropriate credentials

## Installation

1. Clone the repository:
```bash
git clone https://github.com/kgantsov/synconik.git
cd synconik
```

2. Install dependencies:
```bash
go mod download
```

3. Build the project:
```bash
go build
```

## Configuration

The application requires configuration for:
- Iconik credentials (URL, AppID, Token)
- Storage directory for BadgerDB
- Logging settings

Create a configuration file with the following structure:
```yaml
iconik:
  url: "your-iconik-url"
  app_id: "your-app-id"
  token: "your-token"

store:
  data_dir: "/path/to/storage"

log:
  level: "info"
```

## Usage

Run the application:
```bash
./synconik
```

The application will:
1. Initialize the BadgerDB store
2. Start the file scanner to monitor specified directories
3. Begin processing upload jobs to Iconik
4. Handle graceful shutdown on SIGINT/SIGTERM signals

## Project Structure

```
.
├── main.go           # Application entry point
├── internal/
│   ├── config/      # Configuration management
│   ├── iconik/      # Iconik API client
│   ├── scanner/     # File system scanner
│   ├── store/       # BadgerDB storage implementation
│   ├── uploader/    # File upload management
│   ├── entity/      # Core domain entities
│   └── usecase/     # Business logic
└── data/            # Data directory
```

## License

[Add your license information here]

## Contributing

[Add contribution guidelines here]
