# GoSync

**A next-generation sync client that unifies multiple S3 storage backends under a virtual filesystem with intelligent tag-based organization and dynamic filtering.**

> [!WARNING]
> This project is currently in **early development** and is **not ready for production use**. \
> Core components like the gRPC server, testing infrastructure, and production storage backends are incomplete. \
> Do not attempt to use this in any production or critical environment. \
> This `README.md`, as well as all docs have been created with AI. \
> Since the project is still in its initial state with everything open to changes.

---

## Project Description

GoSync is an agent-based sync client that creates a unified virtual filesystem across multiple S3-compatible storage backends (MinIO, AWS S3, Backblaze B2, etc.) while solving the fundamental limitation of hierarchical filesystems. Through a powerful tag-based organization system and dynamic filters, files can be organized by unlimited dimensions simultaneously—like smart playlists for your entire storage infrastructure. Files are stored once but accessible through multiple logical views, with seamless synchronization between local paths, physical backends, and filtered collections.

---

## Table of Contents

- [Why GoSync?](#why-gosync)
- [Key Features](#key-features)
- [Architecture Overview](#architecture-overview)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [Usage Examples](#usage-examples)
- [Installation](#installation)
- [Configuration](#configuration)
- [Command Reference](#command-reference)
- [Use Cases](#use-cases)
- [Development](#development)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Why GoSync?

### The Problem

Traditional filesystems force you to choose **one location** for each file:

```
Where should vacation-sunset.jpg live?
  /photos/2024/vacation/?
  /photos/red-sunsets/?
  /photos/favourites/?
  
You can only pick ONE, but it belongs to ALL of them!
```

Existing solutions either:
- **Duplicate files** across folders (wasting space)
- **Use symbolic links** (breaks on different systems)
- **Lock you into proprietary clouds** (vendor lock-in)
- **Force hierarchical thinking** (limiting organization)

### The Solution

GoSync provides:
- ✅ **Virtual filesystem** unifying multiple S3 backends
- ✅ **Tag-based organization** enabling multi-dimensional views
- ✅ **Dynamic filters** that automatically update
- ✅ **Smart mirroring** syncing filtered collections
- ✅ **Self-hosted** with no vendor lock-in
- ✅ **Files stored once** but accessible through unlimited views

---

## Key Features

### 🗂️ Virtual Filesystem
- **Unified namespace** across multiple S3-compatible backends
- **Dynamic provisioning** of storage backends at runtime
- **Cross-backend operations** like mirroring and syncing
- **Transparent multi-cloud** mixing AWS, Backblaze, MinIO, etc.

### 🏷️ Tag-Based Organization
- **Unlimited tags** per file with key-value pairs
- **Multi-dimensional access** to the same file
- **No duplication** - files exist once, accessible everywhere
- **Auto-tagging** from EXIF, AI, file metadata

### 🔍 Dynamic Filters
- **Query-based virtual paths** like smart playlists
- **Real-time evaluation** as tags change
- **Complex queries** with AND/OR/NOT logic
- **Cross-backend filtering** across all storage

### 🔄 Intelligent Syncing
- **Bidirectional sync** between any path types
- **Filter mirroring** syncing filtered collections locally
- **Backend-to-backend** for automated backups
- **Selective sync** no forced full downloads

### 🗄️ Flexible Metadata Storage
- **SQLite** for simple single-user deployments
- **PostgreSQL** for multi-client coordination
- **Redis cache** for performance optimization
- **Encrypted credentials** for security

### 🛠️ Agent-Based Architecture
- **Single binary** for agent and CLI
- **Unix socket RPC** for command communication
- **Service container** with dependency injection
- **Systemd integration** for daemon operation

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                 Virtual Filesystem                      │
│                                                         │
│  /                                                      │
│  ├── selfhosted/          ◄─ Physical Backends          │
│  │   ├── pictures/                                      │
│  │   └── documents/                                     │
│  ├── aws/                                               │
│  │   └── backups/                                       │
│  ├── backblaze/                                         │
│  │   └── cold-storage/                                  │
│  └── filters/             ◄─ Dynamic Query Views        │
│      ├── pictures/                                      │
│      │   ├── red/         [tag:colour=red]              │
│      │   ├── vacation/    [tag:event=vacation]          │
│      │   └── favourites/  [tag:rating>=4]               │
│      └── work/                                          │
│          ├── urgent/      [tag:priority=high]           │
│          └── active/      [tag:status=active]           │
│                                                         │
└──────────────┬──────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────┐
│         Metadata Database + Tag System                  │
│                                                         │
│  • File metadata (size, dates, checksums)               │
│  • Tags (key-value pairs per file)                      │
│  • Filter definitions (query expressions)               │
│  • Sync states (per client, per backend)                │
│  • Backend configurations (encrypted credentials)       │
│                                                         │
└──────────────┬──────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────┐
│              Physical S3 Backends                       │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   MinIO      │  │   AWS S3     │  │  Backblaze   │   │
│  │  (Local)     │  │  (Cloud)     │  │     B2       │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                         │
│  Files stored as-is, tags in metadata only              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## Quick Start

### 1. Install GoSync

```bash
# Download latest release
curl -L https://github.com/mwantia/gosync/releases/latest/download/gosync -o gosync
chmod +x gosync
sudo mv gosync /usr/local/bin/

# Or build from source
git clone https://github.com/mwantia/gosync
cd gosync
task build
```

### 2. Initialize Configuration

```bash
# Generate default config
gosync config init

# Edit configuration
nano ~/.gosync/config.yaml
```

### 3. Start the Agent

```bash
# Start agent daemon
gosync agent --config ~/.gosync/config.yaml &

# Or install as systemd service
sudo cp gosync.service /etc/systemd/system/
sudo systemctl enable gosync
sudo systemctl start gosync
```

### 4. Provision Your First Backend

```bash
# Add a local MinIO instance
gosync provision selfhosted \
  --endpoint localhost:9000 \
  --bucket sync-bucket \
  --access-key minioadmin \
  --secret-key minioadmin \
  --no-ssl

# List backends
gosync ls
```

### 5. Create Your First Mirror

```bash
# Sync local folder to backend
gosync mirror ~/Documents selfhosted/documents

# Check sync status
gosync sync state
```

---

## Core Concepts

### Backends

**Physical storage locations** (S3-compatible):

```bash
gosync provision selfhosted --endpoint localhost:9000 --bucket sync
gosync provision aws --endpoint s3.amazonaws.com --bucket backup
gosync provision b2 --endpoint s3.us-west-002.backblazeb2.com --bucket archive
```

Access via: `selfhosted/path`, `aws/path`, `b2/path`

### Tags

**Key-value metadata** attached to files:

```bash
# Add tags
gosync tag add selfhosted/photo.jpg \
  colour=red event=vacation year=2024 rating=5

# Search by tags
gosync tag search colour=red year=2024
```

### Filters

**Dynamic virtual paths** based on queries:

```bash
# Create filter
gosync filter create filters/photos/red \
  --filter "tag:colour=red"

# List matches (updates automatically)
gosync ls filters/photos/red

# Mirror filtered collection
gosync mirror ~/Desktop/RedPhotos filters/photos/red
```

### Syncs

**Bidirectional mirroring** between paths:

```bash
# Local ↔ Backend
gosync mirror ~/Documents selfhosted/docs

# Backend ↔ Backend  
gosync mirror-remote selfhosted/important aws/backup/important

# Filter ↔ Local (dynamic!)
gosync mirror ~/Favourites filters/photos/favourites
```

---

## Usage Examples

### Example 1: Photo Organization

```bash
# Provision storage
gosync provision photos --endpoint s3.amazonaws.com --bucket my-photos

# Sync photos from camera
gosync mirror ~/Pictures/Camera photos/camera

# Auto-tag with AI
gosync tag auto photos/camera/ --ai-labels

# Create filters
gosync filter create filters/photos/vacation --filter "tag:event=vacation"
gosync filter create filters/photos/family --filter "tag:people contains 'family'"
gosync filter create filters/photos/red --filter "tag:colour=red"
gosync filter create filters/photos/best --filter "tag:rating>=4"

# Mirror favourites locally
gosync mirror ~/Desktop/BestPhotos filters/photos/best

# Browse by any dimension
gosync ls filters/photos/vacation
gosync ls filters/photos/family
gosync ls filters/photos/red
```

### Example 2: Work Document Management

```bash
# Provision work backend
gosync provision work --endpoint company-minio:9000 --bucket work-docs

# Sync local documents
gosync mirror ~/Documents work/documents

# Tag documents
gosync tag add work/documents/contract.pdf \
  category=legal priority=high project=alpha status=active

# Create smart folders
gosync filter create filters/work/urgent \
  --filter "tag:priority=high AND tag:status=active"

gosync filter create filters/work/project-alpha \
  --filter "tag:project=alpha"

# Mirror urgent items to desktop
gosync mirror ~/Desktop/Urgent filters/work/urgent
```

### Example 3: Multi-Cloud Backup Strategy

```bash
# Provision multiple backends
gosync provision primary --endpoint minio.local:9000 --bucket sync
gosync provision backup --endpoint s3.amazonaws.com --bucket backup
gosync provision archive --endpoint b2.backblazeb2.com --bucket cold

# Tag files by importance
gosync tag add primary/critical/* backup=critical
gosync tag add primary/important/* backup=important
gosync tag add primary/normal/* backup=normal

# Create priority filters
gosync filter create filters/backup/critical --filter "tag:backup=critical"
gosync filter create filters/backup/important --filter "tag:backup=important"

# Set up cascading backups with different frequencies
gosync mirror-remote filters/backup/critical backup/hourly
gosync mirror-remote filters/backup/important backup/daily
gosync mirror-remote filters/backup/normal archive/weekly
```

### Example 4: Media Server Integration

```bash
# Provision media storage
gosync provision media --endpoint s3.amazonaws.com --bucket media-library

# Tag media files
gosync tag add media/movies/*.mkv quality=4k year=2024
gosync tag add media/movies/scifi/* genre=scifi

# Create quality filters
gosync filter create filters/movies/4k --filter "tag:quality=4k"
gosync filter create filters/movies/scifi --filter "tag:genre=scifi"
gosync filter create filters/movies/recent --filter "tag:year>=2020"

# Mirror to Plex library
gosync mirror /mnt/plex/4k filters/movies/4k
gosync mirror /mnt/plex/scifi filters/movies/scifi
```

---

## Installation

### Prerequisites

- Go 1.24 or later (for building from source)
- S3-compatible storage (MinIO, AWS S3, Backblaze B2, etc.)
- PostgreSQL or SQLite for metadata storage
- (Optional) Redis for caching

### From Binary

```bash
# Download latest release
curl -L https://github.com/mwantia/gosync/releases/latest/download/gosync-linux-amd64 -o gosync
chmod +x gosync
sudo mv gosync /usr/local/bin/
```

### From Source

```bash
# Clone repository
git clone https://github.com/mwantia/gosync
cd gosync

# Install dependencies
go mod download

# Build
go build -o gosync cmd/gosync/main.go

# Or use Task
task build
```

### Docker

```bash
# Run with Docker
docker run -d \
  --name gosync \
  -v /path/to/config:/config \
  -v /path/to/data:/data \
  gosync/gosync:latest
```

### Systemd Service

```bash
# Copy service file
sudo cp gosync.service /etc/systemd/system/

# Edit configuration path
sudo nano /etc/systemd/system/gosync.service

# Enable and start
sudo systemctl enable gosync
sudo systemctl start gosync

# Check status
sudo systemctl status gosync
```

---

## Configuration

### Minimal Configuration (SQLite)

```yaml
data_dir: ~/.gosync

metadata:
  type: sqlite
  sqlite:
    path: ~/.gosync/metadata.db

encrypt:
  secret: "your-32-character-secret-key!"

s3:
  # Default S3 config (optional)
  endpoint: localhost:9000
  bucket: default-sync

log:
  level: info
```

### Advanced Configuration (PostgreSQL + Redis)

```yaml
data_dir: /var/lib/gosync

metadata:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    database: gosync
    user: gosync
    password: ${POSTGRES_PASSWORD}
    sslmode: require

cache:
  enabled: true
  redis:
    host: localhost
    port: 6379
    password: ${REDIS_PASSWORD}
    db: 0
  ttl: 5m

encrypt:
  secret: ${ENCRYPT_SECRET}

sync:
  interval: 60s
  workers: 4
  chunk_size: 5MB

log:
  level: info
  file: /var/log/gosync/gosync.log
  rotation:
    max_size: 100
    max_backups: 5
    max_age: 30
    compress: true
```

See [Configuration Guide](docs/configuration.md) for full options.

---

## Command Reference

### Agent Management

```bash
gosync agent [--config <path>]           # Start agent daemon
gosync config init                       # Initialize configuration
gosync config validate                   # Validate configuration
gosync version                           # Show version
```

### Virtual Filesystem

```bash
gosync ls [path]                         # List filesystem contents
gosync ls                                # List all backends
gosync ls selfhosted                     # List backend contents
gosync ls selfhosted/pictures            # List specific path
gosync ls filters/photos/red             # List filter results
gosync ls -lh selfhosted/pictures        # Long format, human-readable
```

### Backend Management

```bash
gosync provision <id> [options]          # Add S3 backend
gosync backend list                      # List all backends
gosync backend show <id>                 # Show backend details
gosync backend update <id> [options]     # Update backend
gosync backend remove <id>               # Remove backend
gosync scan <backend-id>                 # Scan backend metadata
```

### Tag Management

```bash
gosync tag add <path> <key>=<value>...   # Add tags to file
gosync tag list <path>                   # List file tags
gosync tag remove <path> <key>...        # Remove tags
gosync tag search <key>=<value>...       # Search by tags
gosync tag auto <path> [--exif|--ai]     # Auto-tag files
```

### Filter Management

```bash
gosync filter create <path> --filter <query>    # Create filter
gosync filter list                              # List all filters
gosync filter show <path>                       # Show filter details
gosync filter update <path> --filter <query>    # Update filter
gosync filter delete <path>                     # Delete filter
gosync filter test <query>                      # Test filter query
```

### Sync Management

```bash
gosync mirror <local> <virtual>          # Create sync mirror
gosync mirror-remote <src> <dst>         # Mirror between backends
gosync sync state [name]                 # Show sync status
gosync sync list                         # List all syncs
gosync sync pause <name>                 # Pause sync
gosync sync resume <name>                # Resume sync
gosync sync remove <name>                # Remove sync
```

---

## Use Cases

### 📸 Personal Photo Library
- Organize 50,000+ photos by multiple dimensions
- Auto-tag with AI for people, objects, scenes
- Create smart collections (vacation, family, red, etc.)
- Mirror favourites locally without duplication

### 💼 Business Document Management
- Multi-dimensional filing (project, priority, status)
- Smart folders for active work items
- Automatic backup of critical documents
- Team collaboration with shared metadata

### 🎬 Media Server Organization
- Tag by quality, genre, year, language
- Smart libraries for Plex/Jellyfin
- Automatic organization as content is added
- Multi-tier storage (hot/warm/cold)

### 💾 Backup Strategy
- Tag files by importance and frequency
- Tiered backup to different providers
- Automated cascading backups
- Cost optimization with filter-based routing

### 🔬 Research Data Management
- Organize datasets by experiment, date, type
- Track analysis status with tags
- Automatic archival of completed work
- Collaborative access with shared storage

---

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/mwantia/gosync
cd gosync

# Install Task (build tool)
go install github.com/go-task/task/v3/cmd/task@latest

# Install dependencies
task setup

# Build
task build

# Run tests
task test

# Run with test config
task run
```

### Project Structure

```
gosync/
├── cmd/gosync/              # Main application entry point
│   ├── main.go
│   └── cli/                 # CLI commands
├── internal/
│   ├── agent/               # Agent daemon
│   ├── config/              # Configuration management
│   └── client/              # RPC client
├── pkg/
│   ├── backend/             # Backend registry
│   ├── metadata/            # Metadata store abstraction
│   ├── storage/             # S3 storage engines
│   ├── cache/               # Redis cache layer
│   ├── log/                 # Logging service
│   └── filter/              # Filter query engine
├── docs/                    # Documentation
├── config.yaml              # Example configuration
├── Taskfile.yml             # Build tasks
└── README.md
```

### Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

---

## Roadmap

### Phase 1: Core Infrastructure ✅
- [x] Agent architecture with service container
- [x] Virtual filesystem foundation
- [x] Backend registry system
- [x] SQLite metadata storage

### Phase 2: Multi-Backend Support 🚧
- [x] PostgreSQL metadata storage
- [x] Redis cache layer
- [ ] Backend provisioning and management
- [ ] Cross-backend operations

### Phase 3: Tag System 📋
- [ ] Tag CRUD operations
- [ ] Tag indexing and search
- [ ] Bulk tagging operations
- [ ] Auto-tagging (EXIF, AI)

### Phase 4: Dynamic Filters 📋
- [ ] Filter query engine
- [ ] Filter CRUD operations
- [ ] Real-time filter evaluation
- [ ] Filter performance optimization

### Phase 5: Sync Engine 📋
- [ ] Bidirectional sync
- [ ] Filter-aware mirroring
- [ ] Conflict resolution
- [ ] Progress tracking

### Phase 6: Advanced Features 🔮
- [ ] Web UI for management
- [ ] Mobile clients
- [ ] Selective sync
- [ ] File versioning
- [ ] Share links
- [ ] Bandwidth management

---

## FAQ

**Q: How is this different from Rclone?**  
A: Rclone provides low-level sync between remotes. GoSync adds a unified virtual filesystem, tag-based organization, and dynamic filtering across all backends. You can organize files by multiple dimensions without duplication.

**Q: Do I need to migrate my existing S3 data?**  
A: No! GoSync works with files as-is on S3. Just provision the backend and scan to populate metadata. Tags are stored separately.

**Q: Can I use multiple databases?**  
A: Yes! Use SQLite for single-user, PostgreSQL for multi-client coordination. Both work with the same agent.

**Q: How do filters stay up-to-date?**  
A: Filters are evaluated in real-time from the metadata database. When tags change, filter results automatically update.

**Q: Can I mirror between filters?**  
A: Filters are read-only query views, but you can mirror them to local paths or physical backends. Perfect for smart collections!

**Q: Is there a web UI?**  
A: Not yet, but it's on the roadmap. Currently CLI and agent-based.

**Q: How are credentials secured?**  
A: All S3 credentials are encrypted at rest using AES-256 with your configured encryption key.

---

## License

GoSync is released under the [Apache License 2.0](LICENSE).

---

## Acknowledgments

- Built with [fabric](https://github.com/mwantia/fabric) service container
- Inspired by smart playlists, Gmail labels, and the need for better file organization
- Thanks to the Go community for excellent S3 libraries

---

## Support

For issues and questions:
- Open an issue on GitHub
- Check existing issues for solutions
- Review logs at `~/.minio-sync/`

---

**Note**: This is a True Sync Client that maintains a full local copy. For large datasets where you don't need all files locally, consider implementing selective sync (planned feature).