# GoSync Client

A production-ready, cross-platform sync client for S3 (MinIO) that provides true bidirectional synchronization similar to MegaSync, Dropbox, or OneDrive.

## Features

- ✅ **True Sync Client** - Full local copy with background synchronization
- ✅ **Bidirectional Sync** - Changes sync both ways (local ↔ MinIO)
- ✅ **Real-time Monitoring** - Instant detection of local file changes
- ✅ **Concurrent Operations** - Worker pool for fast uploads/downloads
- ✅ **Conflict Resolution** - Last-write-wins strategy
- ✅ **Persistent State** - SQLite database tracks sync status
- ✅ **Ignore Patterns** - Skip unwanted files (.git, .DS_Store, etc.)
- ✅ **Cross-Platform** - Works on Windows, Linux, and macOS
- ✅ **Graceful Shutdown** - Safe handling of interrupts
- ✅ **Extensive Logging** - Debug and track all operations

## Architecture

```
┌─────────────────┐
│  Local Folder   │
└────────┬────────┘
         │
    ┌────▼─────┐
    │  Watcher │────► Detects changes
    └────┬─────┘
         │
    ┌────▼─────┐
    │  Engine  │────► Orchestrates sync
    └────┬─────┘
         │
    ┌────▼─────┐
    │   Queue  │────► Worker pool (4 workers)
    └────┬─────┘
         │
    ┌────▼─────┐
    │  MinIO   │────► S3-compatible storage
    └──────────┘
```

## Installation

### Prerequisites

- Go 1.21 or higher
- MinIO server (or S3-compatible storage)
- MinIO bucket already created

### Build from Source

```bash
# Clone the repository
git clone <your-repo-url>
cd minio-sync

# Download dependencies
go mod download

# Build
go build -o gosync ./cmd/gosync

# Or build for specific platform
GOOS=linux GOARCH=amd64 go build -o gosync-linux ./cmd/gosync
GOOS=windows GOARCH=amd64 go build -o gosync.exe ./cmd/gosync
GOOS=darwin GOARCH=amd64 go build -o gosync-mac ./cmd/gosync
```

## Quick Start

### 1. Initialize Configuration

```bash
./gosync -init
```

This creates `~/.minio-sync/config.yaml` with default settings.

### 2. Edit Configuration

Edit `~/.minio-sync/config.yaml`:

```yaml
local_path: /home/user/MinIOSync  # Your sync folder
minio:
  endpoint: localhost:9000         # MinIO endpoint
  access_key: your-access-key      # Your access key
  secret_key: your-secret-key      # Your secret key
  bucket: sync-bucket              # Bucket name
  use_ssl: false                   # Use HTTPS?
  region: us-east-1                # Region
sync_interval: 60                  # Remote scan interval (seconds)
workers: 4                         # Concurrent workers
chunk_size: 5242880                # 5MB chunks for uploads
ignore_patterns:
  - .git
  - .DS_Store
  - Thumbs.db
  - desktop.ini
  - "*.tmp"
log_level: info                    # debug, info, warn, error
```

### 3. Run the Sync Client

```bash
./gosync
```

Or specify a custom config location:

```bash
./gosync -config /path/to/config.yaml
```

## How It Works

### Initial Sync

When you first start the client:

1. **Scans local folder** - Finds all existing files
2. **Scans MinIO bucket** - Lists all remote files
3. **Compares** - Determines what needs to sync
4. **Syncs** - Downloads missing files, uploads new files
5. **Resolves conflicts** - Uses newest version (last-write-wins)

### Continuous Sync

After initial sync:

1. **File Watcher** monitors local folder for changes
2. **Immediate upload** when you create/modify files locally
3. **Periodic remote scan** checks MinIO every 60 seconds (configurable)
4. **Downloads** new or modified remote files
5. **Deletes** propagate in both directions

### Sync Operations

| Local Action | Result |
|--------------|--------|
| Create file | → Uploads to MinIO |
| Modify file | → Uploads new version |
| Delete file | → Deletes from MinIO |
| Rename file | → Detected as delete + create |

| Remote Action | Result |
|---------------|--------|
| Upload file | → Downloads to local |
| Modify file | → Downloads new version |
| Delete file | → Deletes from local |

### Conflict Resolution

When the same file is modified in both places:

- **Last-write-wins** - Most recent modification time wins
- Warning logged for manual review if needed
- Future: Add conflict resolution strategies (keep both, manual, etc.)

## Project Structure

```
minio-sync/
├── cmd/
│   └── gosync/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── storage/
│   │   ├── minio.go             # MinIO client
│   │   └── local.go             # Local filesystem
│   ├── state/
│   │   └── database.go          # SQLite state management
│   └── sync/
│       ├── engine.go            # Sync orchestration
│       ├── watcher.go           # File watcher
│       └── queue.go             # Worker queue
├── go.mod
└── README.md
```

## Advanced Usage

### Running as a Service

#### Linux (systemd)

Create `/etc/systemd/system/minio-sync.service`:

```ini
[Unit]
Description=MinIO Sync Client
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/gosync -config /home/youruser/.minio-sync/config.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable minio-sync
sudo systemctl start minio-sync
sudo systemctl status minio-sync
```

#### Windows

Use NSSM (Non-Sucking Service Manager):

```powershell
# Download NSSM from https://nssm.cc/
nssm install MinIOSync "C:\path\to\gosync.exe"
nssm set MinIOSync AppDirectory "C:\path\to\"
nssm start MinIOSync
```

### Multiple Sync Folders

Run multiple instances with different configs:

```bash
# Instance 1
./gosync -config ~/.minio-sync/work-config.yaml

# Instance 2  
./gosync -config ~/.minio-sync/personal-config.yaml
```

### Debugging

Enable debug logging in config:

```yaml
log_level: debug
```

Or set environment variable:

```bash
LOG_LEVEL=debug ./gosync
```

## Performance Tips

1. **Adjust worker count** - More workers = faster sync (use 4-8 for SSDs)
2. **Increase chunk size** - Larger chunks for fast networks (5-20MB)
3. **Network bandwidth** - MinIO transfer speeds depend on your connection
4. **Ignore patterns** - Exclude large folders you don't need synced

## Troubleshooting

### "Failed to connect to MinIO"

- Check `endpoint` in config (don't include http://)
- Verify MinIO is running: `curl http://localhost:9000/minio/health/live`
- Check firewall rules

### "Bucket does not exist"

- Create bucket first using MinIO Console or mc client:
  ```bash
  mc mb myminio/sync-bucket
  ```

### Files not syncing

- Check logs for errors
- Verify file isn't in `ignore_patterns`
- Check disk space on both sides
- Look at sync state: `sqlite3 ~/.minio-sync/sync.db "SELECT * FROM file_state;"`

### High CPU usage

- Reduce worker count in config
- Increase `sync_interval` to reduce remote scans
- Add more ignore patterns for frequently changing files

## Limitations

- **No selective sync yet** - All files in bucket are synced (coming soon)
- **Last-write-wins conflicts** - No manual conflict resolution UI yet
- **No encryption** - Files stored as-is (use MinIO SSE if needed)
- **No bandwidth throttling** - Uses full available bandwidth

## Roadmap

- [ ] Selective sync (choose which folders to sync)
- [ ] Web UI for configuration and monitoring
- [ ] Conflict resolution strategies
- [ ] Bandwidth throttling
- [ ] File versioning support
- [ ] Multiple MinIO endpoints
- [ ] Windows shell integration (explorer overlay icons)
- [ ] macOS Finder integration
- [ ] Progress notifications
- [ ] Pause/resume sync

## Development

### Running Tests

```bash
go test ./...
```

### Building for Development

```bash
go build -race -o gosync ./cmd/gosync
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Acknowledgments

- MinIO team for the excellent Go SDK
- fsnotify for cross-platform file watching
- SQLite for reliable state management

## Support

For issues and questions:
- Open an issue on GitHub
- Check existing issues for solutions
- Review logs at `~/.minio-sync/`

---

**Note**: This is a True Sync Client that maintains a full local copy. For large datasets where you don't need all files locally, consider implementing selective sync (planned feature).