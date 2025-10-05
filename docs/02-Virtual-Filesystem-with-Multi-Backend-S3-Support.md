# GoSync: Virtual Filesystem with Multi-Backend S3 Support

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Virtual Filesystem Concept](#virtual-filesystem-concept)
3. [Backend Registry System](#backend-registry-system)
4. [Metadata Schema](#metadata-schema)
5. [Core Commands](#core-commands)
6. [Implementation Guide](#implementation-guide)
7. [Service Container Integration](#service-container-integration)

---

## Architecture Overview

### The Vision

GoSync creates a **virtual filesystem namespace** that unifies access to **multiple S3 storage backends**. Users interact with virtual paths like `selfhosted/pictures` rather than raw S3 URLs, enabling:

- **Multiple backends**: AWS S3, Backblaze B2, local MinIO, etc.
- **Unified interface**: Single namespace across all backends
- **Logical organization**: `backups/`, `media/`, `archives/` as separate backends
- **Flexible mirroring**: Sync between backends or local paths
- **Dynamic provisioning**: Add backends at runtime

```
┌─────────────────────────────────────────────────────────┐
│              Virtual Filesystem Layer                   │
│                                                         │
│  /                         (root)                       │
│  ├── selfhosted/          (Backend 1: Local MinIO)      │
│  │   ├── pictures/                                      │
│  │   └── documents/                                     │
│  ├── aws/                 (Backend 2: AWS S3)           │
│  │   ├── backups/                                       │
│  │   └── archives/                                      │
│  └── backblaze/           (Backend 3: Backblaze B2)     │
│      └── cold-storage/                                  │
│                                                         │
└──────────────┬──────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────┐
│            Backend Registry (Metadata DB)               │
│                                                         │
│  Backend: "selfhosted"                                  │
│    - endpoint: minio.local:9000                         │
│    - bucket: sync-bucket                                │
│    - credentials: (encrypted)                           │
│                                                         │
│  Backend: "aws"                                         │
│    - endpoint: s3.amazonaws.com                         │
│    - bucket: company-backup                             │
│    - credentials: (encrypted)                           │
│                                                         │
└──────────────┬──────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────────────────────┐
│              Physical S3 Backends                       │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   MinIO      │  │   AWS S3     │  │  Backblaze   │   │
│  │  Local Net   │  │  us-east-1   │  │   B2 Cloud   │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Key Innovation: Virtual Path Resolution

```
User Command: gosync ls selfhosted/pictures
                 ↓
    Parse Virtual Path
                 ↓
Backend: "selfhosted"  |  Path: "pictures"
                 ↓
   Resolve Backend Config
                 ↓
S3 Client: minio.local:9000, bucket=sync-bucket
                 ↓
    Query Metadata: WHERE backend_id='selfhosted' AND path LIKE 'pictures/%'
                 ↓
    Display Results
```

---

## Virtual Filesystem Concept

### Design Philosophy

Instead of directly working with S3 buckets, users interact with a **logical namespace**:

```bash
# Traditional approach (complex)
aws s3 ls s3://my-bucket/photos --endpoint-url=http://minio:9000
rclone ls backblaze:my-backup/documents

# GoSync approach (unified)
gosync ls selfhosted/photos
gosync ls backblaze/documents
```

### Benefits

1. **Abstraction**: Hide S3 complexity behind simple paths
2. **Multi-Cloud**: Mix providers transparently
3. **Organization**: Logical structure independent of physical storage
4. **Flexibility**: Mirror between any backends
5. **Portability**: Change backends without changing workflows

### Virtual Path Structure

```
<backend>/<path>
   ↓        ↓
   |        └─ Path within that backend's S3 bucket
   └────────── Backend name (logical identifier)

Examples:
- selfhosted/pictures/vacation.jpg
- aws/backups/database.sql.gz
- backblaze/archives/2024/Q1/data.tar
```

### Root Listing Behavior

```bash
$ gosync ls
Total 3
root          gosync gosync  0 .
provisioning  xxx    xxx     ? selfhosted
provisioning  xxx    xxx     ? aws  
ready         gosync gosync  0 backblaze
```

**Status meanings:**
- `provisioning`: Backend added but not yet scanned
- `ready`: Backend fully provisioned and operational
- `error`: Backend configuration issue

---

## Backend Registry System

### Backend Model

```go
// pkg/backend/backend.go
package backend

import (
    "time"
    "github.com/mwantia/gosync/pkg/storage"
)

type Backend struct {
    ID          string    `json:"id"`           // e.g., "selfhosted"
    Name        string    `json:"name"`         // Display name
    Type        string    `json:"type"`         // "s3", "minio", etc.
    Status      string    `json:"status"`       // "provisioning", "ready", "error"
    Config      S3Config  `json:"config"`       // S3 configuration
    Stats       BackendStats `json:"stats"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    ProvisionedAt *time.Time `json:"provisioned_at,omitempty"`
}

type S3Config struct {
    Endpoint  string `json:"endpoint"`
    Region    string `json:"region"`
    Bucket    string `json:"bucket"`
    AccessKey string `json:"access_key"`  // Encrypted at rest
    SecretKey string `json:"secret_key"`  // Encrypted at rest
    UseSSL    bool   `json:"use_ssl"`
    PathStyle bool   `json:"path_style"`
}

type BackendStats struct {
    FileCount      int64   `json:"file_count"`
    TotalSize      int64   `json:"total_size"`
    LastSyncTime   *time.Time `json:"last_sync_time,omitempty"`
    ProvisionProgress float64 `json:"provision_progress"` // 0.0 - 1.0
}

// Registry manages all backends
type Registry interface {
    // Backend management
    Register(backend *Backend) error
    Get(id string) (*Backend, error)
    List() ([]*Backend, error)
    Update(backend *Backend) error
    Delete(id string) error
    
    // Path resolution
    ResolveVirtualPath(path string) (*Backend, string, error)
    
    // Storage client
    GetStorageClient(backendID string) (storage.StorageEngine, error)
}
```

### Backend Registry Implementation

```go
// pkg/backend/registry.go
package backend

import (
    "context"
    "fmt"
    "strings"
    "sync"
    
    "github.com/mwantia/gosync/pkg/metadata"
    "github.com/mwantia/gosync/pkg/storage"
    "github.com/mwantia/gosync/pkg/storage/s3"
)

type RegistryImpl struct {
    metadataStore metadata.MetadataStore
    encryptService EncryptService
    
    // Cache of storage clients
    clients map[string]storage.StorageEngine
    mu      sync.RWMutex
}

func NewRegistry(store metadata.MetadataStore, encrypt EncryptService) *RegistryImpl {
    return &RegistryImpl{
        metadataStore: store,
        encryptService: encrypt,
        clients: make(map[string]storage.StorageEngine),
    }
}

func (r *RegistryImpl) Register(backend *Backend) error {
    // Validate backend ID (must be valid path component)
    if !isValidBackendID(backend.ID) {
        return fmt.Errorf("invalid backend ID: must be alphanumeric with hyphens/underscores")
    }
    
    // Encrypt credentials
    if err := r.encryptCredentials(&backend.Config); err != nil {
        return fmt.Errorf("failed to encrypt credentials: %w", err)
    }
    
    // Store in metadata DB
    return r.metadataStore.UpsertBackend(context.Background(), backend)
}

func (r *RegistryImpl) ResolveVirtualPath(virtualPath string) (*Backend, string, error) {
    // Split virtual path: "selfhosted/pictures/vacation.jpg"
    parts := strings.SplitN(virtualPath, "/", 2)
    if len(parts) == 0 {
        return nil, "", fmt.Errorf("invalid virtual path")
    }
    
    backendID := parts[0]
    backendPath := ""
    if len(parts) > 1 {
        backendPath = parts[1]
    }
    
    // Lookup backend
    backend, err := r.Get(backendID)
    if err != nil {
        return nil, "", fmt.Errorf("backend not found: %s", backendID)
    }
    
    return backend, backendPath, nil
}

func (r *RegistryImpl) GetStorageClient(backendID string) (storage.StorageEngine, error) {
    r.mu.RLock()
    if client, exists := r.clients[backendID]; exists {
        r.mu.RUnlock()
        return client, nil
    }
    r.mu.RUnlock()
    
    // Create new client
    backend, err := r.Get(backendID)
    if err != nil {
        return nil, err
    }
    
    // Decrypt credentials
    config := backend.Config
    if err := r.decryptCredentials(&config); err != nil {
        return nil, err
    }
    
    // Create S3 client
    client, err := s3.NewS3Storage(config)
    if err != nil {
        return nil, err
    }
    
    // Cache it
    r.mu.Lock()
    r.clients[backendID] = client
    r.mu.Unlock()
    
    return client, nil
}

func isValidBackendID(id string) bool {
    // Only allow alphanumeric, hyphens, and underscores
    // Must not start with dot or slash
    if len(id) == 0 || id[0] == '.' || id[0] == '/' {
        return false
    }
    
    for _, c := range id {
        if !((c >= 'a' && c <= 'z') || 
             (c >= 'A' && c <= 'Z') || 
             (c >= '0' && c <= '9') || 
             c == '-' || c == '_') {
            return false
        }
    }
    
    return true
}

func (r *RegistryImpl) encryptCredentials(config *S3Config) error {
    var err error
    config.AccessKey, err = r.encryptService.Encrypt(config.AccessKey)
    if err != nil {
        return err
    }
    
    config.SecretKey, err = r.encryptService.Encrypt(config.SecretKey)
    if err != nil {
        return err
    }
    
    return nil
}

func (r *RegistryImpl) decryptCredentials(config *S3Config) error {
    var err error
    config.AccessKey, err = r.encryptService.Decrypt(config.AccessKey)
    if err != nil {
        return err
    }
    
    config.SecretKey, err = r.encryptService.Decrypt(config.SecretKey)
    if err != nil {
        return err
    }
    
    return nil
}
```

---

## Metadata Schema

### Enhanced Schema with Backends

```sql
-- Backend registry table
CREATE TABLE backends (
    id TEXT PRIMARY KEY,                    -- e.g., "selfhosted", "aws"
    name TEXT NOT NULL,
    type TEXT NOT NULL,                     -- "s3", "minio"
    status TEXT NOT NULL DEFAULT 'provisioning', -- provisioning, ready, error
    
    -- S3 Configuration (encrypted)
    endpoint TEXT NOT NULL,
    region TEXT,
    bucket TEXT NOT NULL,
    access_key TEXT NOT NULL,               -- Encrypted
    secret_key TEXT NOT NULL,               -- Encrypted
    use_ssl BOOLEAN DEFAULT TRUE,
    path_style BOOLEAN DEFAULT FALSE,
    
    -- Statistics
    file_count BIGINT DEFAULT 0,
    total_size BIGINT DEFAULT 0,
    last_sync_time TIMESTAMP,
    provision_progress REAL DEFAULT 0.0,    -- 0.0 to 1.0
    
    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    provisioned_at TIMESTAMP,
    
    CONSTRAINT chk_status CHECK (status IN ('provisioning', 'ready', 'error'))
);

-- Files table with backend reference
CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    backend_id TEXT NOT NULL REFERENCES backends(id) ON DELETE CASCADE,
    
    -- Virtual path (without backend prefix)
    path TEXT NOT NULL,
    
    -- S3 information
    s3_key TEXT NOT NULL,
    size BIGINT NOT NULL,
    etag VARCHAR(64),
    checksum VARCHAR(64),
    
    -- Timestamps
    modified_time TIMESTAMP NOT NULL,
    created_time TIMESTAMP NOT NULL,
    
    -- File metadata
    mime_type VARCHAR(255),
    is_directory BOOLEAN DEFAULT FALSE,
    parent_id BIGINT REFERENCES files(id),
    
    -- Versioning
    version INTEGER DEFAULT 1,
    is_deleted BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMP,
    
    -- Unique constraint per backend
    CONSTRAINT unique_backend_path UNIQUE(backend_id, path)
);

CREATE INDEX idx_files_backend ON files(backend_id);
CREATE INDEX idx_files_parent ON files(parent_id);
CREATE INDEX idx_files_path ON files(backend_id, path);
CREATE INDEX idx_files_deleted ON files(backend_id, is_deleted);

-- Sync configurations with virtual paths
CREATE TABLE sync_configs (
    id BIGSERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    
    -- Local path
    local_path TEXT NOT NULL,
    
    -- Virtual path (includes backend)
    virtual_path TEXT NOT NULL,              -- e.g., "selfhosted/pictures"
    
    -- Configuration
    bidirectional BOOLEAN DEFAULT TRUE,
    ignore_patterns TEXT[],
    status TEXT DEFAULT 'active',
    
    -- Statistics
    last_sync_time TIMESTAMP,
    file_count BIGINT DEFAULT 0,
    
    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_status CHECK (status IN ('active', 'paused', 'error'))
);

-- File changes with backend reference
CREATE TABLE file_changes (
    id BIGSERIAL PRIMARY KEY,
    backend_id TEXT NOT NULL REFERENCES backends(id),
    file_id BIGINT REFERENCES files(id),
    
    change_type VARCHAR(20) NOT NULL,        -- CREATE, UPDATE, DELETE, RENAME
    changed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    client_id UUID,
    
    old_path TEXT,
    new_path TEXT,
    
    CONSTRAINT chk_change_type CHECK (change_type IN ('CREATE', 'UPDATE', 'DELETE', 'RENAME'))
);

CREATE INDEX idx_changes_backend ON file_changes(backend_id, changed_at);
CREATE INDEX idx_changes_file ON file_changes(file_id);

-- Sync state per client per backend
CREATE TABLE sync_states (
    id BIGSERIAL PRIMARY KEY,
    client_id UUID NOT NULL,
    backend_id TEXT NOT NULL REFERENCES backends(id),
    
    last_sync_time TIMESTAMP NOT NULL,
    sync_cursor BIGINT,
    
    device_name TEXT,
    platform TEXT,
    
    CONSTRAINT unique_client_backend UNIQUE(client_id, backend_id)
);
```

---

## Core Commands

### 1. List Command (`gosync ls`)

```go
// cmd/gosync/cli/ls.go
package cli

import (
    "fmt"
    "strings"
    
    "github.com/spf13/cobra"
    "github.com/mwantia/gosync/internal/client"
)

func NewLsCommand() *cobra.Command {
    var long bool
    var humanReadable bool
    
    cmd := &cobra.Command{
        Use:   "ls [path]",
        Short: "List virtual filesystem contents",
        Long: `List files and directories in the virtual filesystem.

Without arguments, lists all backends at root level.
With a path, lists contents of that virtual path.

Examples:
  # List all backends
  gosync ls
  
  # List contents of a backend
  gosync ls selfhosted
  
  # List specific path within backend
  gosync ls selfhosted/pictures
  
  # Long format with details
  gosync ls -l selfhosted/documents
  
  # Human-readable sizes
  gosync ls -lh selfhosted`,
        
        RunE: func(cmd *cobra.Command, args []string) error {
            path := ""
            if len(args) > 0 {
                path = args[0]
            }
            
            client, err := client.NewAgentClient()
            if err != nil {
                return err
            }
            
            return listPath(client, path, long, humanReadable)
        },
    }
    
    cmd.Flags().BoolVarP(&long, "long", "l", false, "use long listing format")
    cmd.Flags().BoolVarP(&humanReadable, "human-readable", "h", false, "human-readable sizes")
    
    return cmd
}

func listPath(client *client.AgentClient, path string, long, human bool) error {
    ctx := context.Background()
    
    // Root level: list backends
    if path == "" || path == "/" {
        backends, err := client.ListBackends(ctx)
        if err != nil {
            return err
        }
        
        fmt.Printf("Total %d\n", len(backends))
        
        if long {
            // Long format
            for _, backend := range backends {
                size := formatSize(backend.Stats.TotalSize, human)
                fmt.Printf("%-12s %-8s %-8s %8s %s\n",
                    backend.Status,
                    "gosync", // owner
                    "gosync", // group
                    size,
                    backend.ID,
                )
            }
        } else {
            // Simple format
            for _, backend := range backends {
                fmt.Println(backend.ID)
            }
        }
        
        return nil
    }
    
    // Backend or deeper path: list files
    entries, err := client.ListFiles(ctx, path)
    if err != nil {
        return err
    }
    
    fmt.Printf("Total %d\n", len(entries))
    
    if long {
        for _, entry := range entries {
            perms := "drwxr-xr-x"
            if !entry.IsDirectory {
                perms = "-rw-r--r--"
            }
            
            size := formatSize(entry.Size, human)
            modTime := entry.ModifiedTime.Format("Jan 02 15:04")
            
            fmt.Printf("%s 1 gosync gosync %8s %s %s\n",
                perms, size, modTime, entry.Name,
            )
        }
    } else {
        for _, entry := range entries {
            name := entry.Name
            if entry.IsDirectory {
                name += "/"
            }
            fmt.Println(name)
        }
    }
    
    return nil
}

func formatSize(size int64, human bool) string {
    if !human {
        return fmt.Sprintf("%d", size)
    }
    
    const unit = 1024
    if size < unit {
        return fmt.Sprintf("%dB", size)
    }
    
    div, exp := int64(unit), 0
    for n := size / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    
    return fmt.Sprintf("%.1f%c", float64(size)/float64(div), "KMGTPE"[exp])
}
```

### 2. Provision Command (`gosync provision`)

```go
// cmd/gosync/cli/provision.go
package cli

import (
    "fmt"
    
    "github.com/spf13/cobra"
    "github.com/mwantia/gosync/internal/client"
    "github.com/mwantia/gosync/pkg/backend"
)

func NewProvisionCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "provision <backend-id>",
        Short: "Provision a new S3 backend",
        Long: `Provision a new S3 storage backend and add it to the virtual filesystem.

This command:
1. Registers the backend configuration
2. Tests the S3 connection
3. Scans the bucket to populate metadata
4. Makes the backend available in virtual filesystem

Examples:
  # Provision a local MinIO instance
  gosync provision selfhosted \
    --endpoint localhost:9000 \
    --bucket sync-bucket \
    --access-key minioadmin \
    --secret-key minioadmin \
    --no-ssl
  
  # Provision AWS S3
  gosync provision aws \
    --endpoint s3.amazonaws.com \
    --region us-east-1 \
    --bucket my-backup \
    --access-key AKIA... \
    --secret-key ...
  
  # Provision with custom name
  gosync provision bb \
    --name "Backblaze B2" \
    --endpoint s3.us-west-002.backblazeb2.com \
    --bucket my-backup \
    --access-key ... \
    --secret-key ...`,
        
        Args: cobra.ExactArgs(1),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            backendID := args[0]
            
            // Get flags
            name, _ := cmd.Flags().GetString("name")
            endpoint, _ := cmd.Flags().GetString("endpoint")
            region, _ := cmd.Flags().GetString("region")
            bucket, _ := cmd.Flags().GetString("bucket")
            accessKey, _ := cmd.Flags().GetString("access-key")
            secretKey, _ := cmd.Flags().GetString("secret-key")
            useSSL, _ := cmd.Flags().GetBool("ssl")
            pathStyle, _ := cmd.Flags().GetBool("path-style")
            scanNow, _ := cmd.Flags().GetBool("scan-now")
            
            // Default name to ID if not provided
            if name == "" {
                name = backendID
            }
            
            // Validate required fields
            if endpoint == "" {
                return fmt.Errorf("--endpoint is required")
            }
            if bucket == "" {
                return fmt.Errorf("--bucket is required")
            }
            if accessKey == "" {
                return fmt.Errorf("--access-key is required")
            }
            if secretKey == "" {
                return fmt.Errorf("--secret-key is required")
            }
            
            // Create backend configuration
            backend := &backend.Backend{
                ID:   backendID,
                Name: name,
                Type: "s3",
                Config: backend.S3Config{
                    Endpoint:  endpoint,
                    Region:    region,
                    Bucket:    bucket,
                    AccessKey: accessKey,
                    SecretKey: secretKey,
                    UseSSL:    useSSL,
                    PathStyle: pathStyle,
                },
            }
            
            // Send to agent
            client, err := client.NewAgentClient()
            if err != nil {
                return err
            }
            
            fmt.Printf("Provisioning backend '%s'...\n", backendID)
            
            if err := client.ProvisionBackend(cmd.Context(), backend, scanNow); err != nil {
                return err
            }
            
            fmt.Printf("✓ Backend '%s' provisioned successfully\n", backendID)
            
            if scanNow {
                fmt.Println("Scanning bucket contents...")
                // Progress will be shown by agent logs
            } else {
                fmt.Println("\nTo scan the bucket contents, run:")
                fmt.Printf("  gosync sync scan %s\n", backendID)
            }
            
            return nil
        },
    }
    
    cmd.Flags().String("name", "", "display name for backend")
    cmd.Flags().String("endpoint", "", "S3 endpoint (required)")
    cmd.Flags().String("region", "us-east-1", "S3 region")
    cmd.Flags().String("bucket", "", "S3 bucket name (required)")
    cmd.Flags().String("access-key", "", "S3 access key (required)")
    cmd.Flags().String("secret-key", "", "S3 secret key (required)")
    cmd.Flags().Bool("ssl", true, "use SSL/TLS")
    cmd.Flags().Bool("path-style", false, "use path-style URLs (for MinIO)")
    cmd.Flags().Bool("scan-now", true, "scan bucket immediately after provisioning")
    
    return cmd
}
```

### 3. Mirror Command with Virtual Paths

```go
// cmd/gosync/cli/mirror.go
package cli

import (
    "fmt"
    "path/filepath"
    
    "github.com/spf13/cobra"
    "github.com/mwantia/gosync/internal/client"
)

func NewMirrorCommand() *cobra.Command {
    var bidirectional bool
    var ignorePatterns []string
    var syncName string
    
    cmd := &cobra.Command{
        Use:   "mirror <local-path> <virtual-path>",
        Short: "Create a sync mirror between local path and virtual path",
        Long: `Create a bidirectional sync between a local directory and a virtual path.

Virtual paths use the format: <backend>/<path>

Examples:
  # Mirror local pictures to selfhosted backend
  gosync mirror ~/Pictures selfhosted/pictures
  
  # Mirror with custom name
  gosync mirror ~/Documents aws/backups/documents --name doc-backup
  
  # Upload-only (no download)
  gosync mirror ~/Backups backblaze/cold-storage --no-bidirectional
  
  # With ignore patterns
  gosync mirror ~/Code selfhosted/projects --ignore "*.log,node_modules,.git"
  
  # Mirror between two backends (yes, this works!)
  gosync mirror-remote aws/primary backblaze/backup`,
        
        Args: cobra.ExactArgs(2),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            localPath := args[0]
            virtualPath := args[1]
            
            // Expand home directory
            if localPath[0] == '~' {
                home, _ := os.UserHomeDir()
                localPath = filepath.Join(home, localPath[1:])
            }
            
            // Validate paths
            if _, err := os.Stat(localPath); err != nil {
                return fmt.Errorf("local path does not exist: %s", localPath)
            }
            
            // Parse virtual path to validate backend exists
            client, err := client.NewAgentClient()
            if err != nil {
                return err
            }
            
            // Auto-generate name if not provided
            if syncName == "" {
                syncName = filepath.Base(localPath)
            }
            
            opts := client.MirrorOptions{
                Name:           syncName,
                LocalPath:      localPath,
                VirtualPath:    virtualPath,
                Bidirectional:  bidirectional,
                IgnorePatterns: ignorePatterns,
            }
            
            fmt.Printf("Creating sync mirror '%s'\n", syncName)
            fmt.Printf("  Local:   %s\n", localPath)
            fmt.Printf("  Virtual: %s\n", virtualPath)
            fmt.Printf("  Mode:    ")
            if bidirectional {
                fmt.Println("bidirectional")
            } else {
                fmt.Println("upload-only")
            }
            
            if err := client.CreateMirror(cmd.Context(), opts); err != nil {
                return err
            }
            
            fmt.Println("✓ Sync mirror created successfully")
            fmt.Println("\nMonitor sync status with:")
            fmt.Printf("  gosync sync state %s\n", syncName)
            
            return nil
        },
    }
    
    cmd.Flags().BoolVar(&bidirectional, "bidirectional", true, "enable bidirectional sync")
    cmd.Flags().StringSliceVar(&ignorePatterns, "ignore", nil, "patterns to ignore")
    cmd.Flags().StringVar(&syncName, "name", "", "custom name for sync")
    
    return cmd
}
```

### 4. Scan Command (Populate Metadata)

```go
// cmd/gosync/cli/scan.go
package cli

import (
    "fmt"
    
    "github.com/spf13/cobra"
    "github.com/mwantia/gosync/internal/client"
)

func NewScanCommand() *cobra.Command {
    var recursive bool
    var dryRun bool
    var prefix string
    
    cmd := &cobra.Command{
        Use:   "scan <backend-id>",
        Short: "Scan backend and populate metadata",
        Long: `Scan an S3 backend to populate the metadata database.

This is useful for:
- Initial provisioning of existing buckets
- Recovering from metadata loss
- Updating metadata after external changes

The scan process:
1. Lists all objects in the S3 bucket
2. Extracts metadata (size, modified time, etc.)
3. Populates the metadata database
4. Does NOT download files

Examples:
  # Scan entire backend
  gosync scan selfhosted
  
  # Scan specific prefix
  gosync scan selfhosted --prefix pictures/
  
  # Dry run (show what would be scanned)
  gosync scan aws --dry-run
  
  # Non-recursive (current directory only)
  gosync scan selfhosted --prefix documents/ --no-recursive`,
        
        Args: cobra.ExactArgs(1),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            backendID := args[0]
            
            client, err := client.NewAgentClient()
            if err != nil {
                return err
            }
            
            opts := client.ScanOptions{
                BackendID: backendID,
                Prefix:    prefix,
                Recursive: recursive,
                DryRun:    dryRun,
            }
            
            fmt.Printf("Scanning backend '%s'", backendID)
            if prefix != "" {
                fmt.Printf(" (prefix: %s)", prefix)
            }
            fmt.Println()
            
            if dryRun {
                fmt.Println("DRY RUN - no changes will be made")
            }
            
            // Start scan (async)
            scanID, err := client.StartScan(cmd.Context(), opts)
            if err != nil {
                return err
            }
            
            fmt.Printf("Scan started (ID: %s)\n", scanID)
            fmt.Println("\nMonitor progress with:")
            fmt.Printf("  gosync scan status %s\n", scanID)
            
            return nil
        },
    }
    
    cmd.Flags().BoolVar(&recursive, "recursive", true, "scan recursively")
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be scanned")
    cmd.Flags().StringVar(&prefix, "prefix", "", "only scan this prefix")
    
    return cmd
}
```

### Complete Command Tree

```
gosync
├── agent                       # Run agent daemon
│   └── --config <path>
│
├── ls [path]                   # List virtual filesystem
│   ├── -l                      # Long format
│   └── -h                      # Human-readable sizes
│
├── provision <backend-id>      # Add new S3 backend
│   ├── --endpoint <url>
│   ├── --bucket <name>
│   ├── --access-key <key>
│   ├── --secret-key <key>
│   ├── --region <region>
│   ├── --ssl
│   └── --scan-now
│
├── scan <backend-id>           # Scan backend metadata
│   ├── --prefix <path>
│   ├── --recursive
│   └── --dry-run
│
├── mirror <local> <virtual>    # Create sync mirror
│   ├── --name <name>
│   ├── --ignore <patterns>
│   └── --no-bidirectional
│
├── sync                        # Sync management
│   ├── state [name]            # Show sync state
│   ├── list                    # List all syncs
│   ├── pause <name>            # Pause sync
│   ├── resume <name>           # Resume sync
│   └── remove <name>           # Remove sync
│
├── backend                     # Backend management
│   ├── list                    # List backends
│   ├── show <id>               # Show backend details
│   ├── update <id>             # Update backend config
│   └── remove <id>             # Remove backend
│
└── config                      # Configuration
    ├── init                    # Initialize config
    ├── validate                # Validate config
    └── show                    # Show current config
```

---

## Implementation Guide

### Phase 1: Backend Registry (Week 1)

**Goals:**
- Backend model and schema
- Registry implementation
- Virtual path resolution
- Encryption service

**Tasks:**
```go
// 1. Define backend model
type Backend struct {
    ID     string
    Config S3Config
    Status string
    Stats  BackendStats
}

// 2. Implement registry
type Registry interface {
    Register(backend *Backend) error
    Get(id string) (*Backend, error)
    ResolveVirtualPath(path string) (*Backend, string, error)
    GetStorageClient(backendID string) (storage.StorageEngine, error)
}

// 3. Add encryption service
type EncryptService interface {
    Encrypt(plaintext string) (string, error)
    Decrypt(ciphertext string) (string, error)
}

// 4. Update metadata schema
ALTER TABLE files ADD COLUMN backend_id TEXT REFERENCES backends(id);
```

### Phase 2: Core Commands (Week 2)

**Goals:**
- LS command
- Provision command
- Scan command
- Virtual path handling

**Tasks:**
1. Implement `gosync ls` with root and path listing
2. Implement `gosync provision` to register backends
3. Implement `gosync scan` to populate metadata
4. Add progress tracking for scans
5. Test with multiple backends

### Phase 3: Mirror with Virtual Paths (Week 3)

**Goals:**
- Update mirror command for virtual paths
- Path resolution in sync engine
- Multi-backend sync support

**Tasks:**
```go
// 1. Update SyncConfig
type SyncConfig struct {
    LocalPath    string
    VirtualPath  string  // e.g., "selfhosted/pictures"
    BackendID    string  // Resolved from virtual path
    BackendPath  string  // Path within backend
}

// 2. Update sync engine
func (e *Engine) ResolvePaths(config *SyncConfig) error {
    backend, path, err := e.registry.ResolveVirtualPath(config.VirtualPath)
    config.BackendID = backend.ID
    config.BackendPath = path
    return err
}

// 3. Update file operations
func (e *Engine) UploadFile(localPath string, sync *SyncConfig) error {
    client, err := e.registry.GetStorageClient(sync.BackendID)
    // Use resolved client and path
}
```

### Phase 4: Multi-Backend Support (Week 4)

**Goals:**
- Backend switching in sync engine
- Per-backend sync states
- Cross-backend mirroring

**Tasks:**
1. Support multiple active backends
2. Track sync state per backend
3. Enable backend-to-backend syncs
4. Add backend status monitoring

### Phase 5: Advanced Features (Week 5+)

**Goals:**
- Backend health checks
- Automatic failover
- Backend statistics
- Web UI

**Tasks:**
1. Periodic backend health checks
2. Mark backends offline when unreachable
3. Collect per-backend statistics
4. Build web UI for visualization

---

## Service Container Integration

Following your raindrop pattern with fabric container:

```go
// internal/agent/agent.go
package agent

import (
    "context"
    
    "github.com/mwantia/fabric/pkg/container"
    "github.com/mwantia/gosync/internal/config"
    "github.com/mwantia/gosync/pkg/backend"
    "github.com/mwantia/gosync/pkg/cache"
    "github.com/mwantia/gosync/pkg/log"
    "github.com/mwantia/gosync/pkg/metadata"
    "github.com/mwantia/gosync/pkg/storage"
)

type Agent struct {
    config *config.Config
    sc     *container.ServiceContainer
    log    log.LoggerService
}

func NewAgent(cfg *config.Config) (*Agent, error) {
    agent := &Agent{
        config: cfg,
        sc:     container.NewServiceContainer(),
        log:    log.NewLoggerService("gosync", cfg.Log),
    }
    
    if err := agent.setupServices(); err != nil {
        return nil, err
    }
    
    return agent, nil
}

func (a *Agent) setupServices() error {
    errs := container.Errors{}
    
    // Logger service
    a.log.Debug("Registering LoggerService...")
    errs.Add(container.Register[log.LoggerServiceImpl](a.sc,
        container.With[log.LoggerService](),
        container.WithInstance(a.log)))
    
    // Encryption service
    a.log.Debug("Registering EncryptService...")
    errs.Add(container.Register[EncryptServiceImpl](a.sc,
        container.With[EncryptService](),
        container.WithFactory(func() (EncryptService, error) {
            return NewEncryptService(a.config.Encrypt.Secret)
        })))
    
    // Metadata store (factory based on config)
    a.log.Debug("Registering MetadataStore...")
    errs.Add(container.Register[metadata.MetadataStore](a.sc,
        container.WithFactory(func() (metadata.MetadataStore, error) {
            return metadata.NewMetadataStore(a.config.Metadata)
        })))
    
    // Cache layer (optional)
    if a.config.Cache.Enabled {
        a.log.Debug("Registering CacheLayer...")
        errs.Add(container.Register[cache.CacheLayer](a.sc,
            container.WithFactory(func() (cache.CacheLayer, error) {
                return cache.NewCacheLayer(a.config.Cache)
            })))
    }
    
    // Backend registry
    a.log.Debug("Registering BackendRegistry...")
    errs.Add(container.Register[backend.RegistryImpl](a.sc,
        container.With[backend.Registry](),
        container.WithFactory(func() (*backend.RegistryImpl, error) {
            store := container.Resolve[metadata.MetadataStore](a.sc)
            encrypt := container.Resolve[EncryptService](a.sc)
            return backend.NewRegistry(store, encrypt), nil
        })))
    
    // Sync engine
    a.log.Debug("Registering SyncEngine...")
    errs.Add(container.Register[SyncEngineImpl](a.sc,
        container.With[SyncEngine](),
        container.WithFactory(func() (*SyncEngineImpl, error) {
            registry := container.Resolve[backend.Registry](a.sc)
            store := container.Resolve[metadata.MetadataStore](a.sc)
            logger := container.Resolve[log.LoggerService](a.sc)
            
            return NewSyncEngine(registry, store, logger, a.config.Sync), nil
        })))
    
    // RPC server
    a.log.Debug("Registering RPC Server...")
    errs.Add(container.Register[AgentRPC](a.sc,
        container.WithFactory(func() (*AgentRPC, error) {
            return NewAgentRPC(a, a.config.DataDir), nil
        })))
    
    return errs.Errors()
}

func (a *Agent) Run(ctx context.Context) error {
    // Start RPC server
    rpc := container.Resolve[AgentRPC](a.sc)
    go rpc.Start(ctx)
    
    // Start sync engine
    engine := container.Resolve[SyncEngine](a.sc)
    go engine.Run(ctx)
    
    // Wait for shutdown
    <-ctx.Done()
    
    // Cleanup
    return a.sc.Cleanup(context.Background())
}
```

---

## Configuration Example

```yaml
# config.yaml - GoSync with Virtual Filesystem

data_dir: /var/lib/gosync

# Encryption for credentials storage
encrypt:
  secret: "your-32-character-secret-key!"

# Metadata store
metadata:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    database: gosync
    user: gosync
    password: ${POSTGRES_PASSWORD}

# Optional Redis cache
cache:
  enabled: true
  redis:
    host: localhost
    port: 6379
  ttl: 5m

# Logging
log:
  level: info
  file: /var/log/gosync/gosync.log

# Sync engine settings
sync:
  interval: 60s
  workers: 4
  chunk_size: 5MB

# Backends can also be defined in config (optional)
# Usually added dynamically via 'gosync provision'
backends:
  - id: selfhosted
    name: "Local MinIO"
    endpoint: localhost:9000
    bucket: sync-bucket
    access_key: ${MINIO_ACCESS_KEY}
    secret_key: ${MINIO_SECRET_KEY}
    use_ssl: false
    path_style: true

# Sync configurations with virtual paths
syncs:
  - name: documents
    local_path: ~/Documents
    virtual_path: selfhosted/documents
    bidirectional: true
  
  - name: photos-backup
    local_path: ~/Pictures
    virtual_path: aws/backups/pictures
    bidirectional: false  # Upload only
```

---

## Usage Examples

### Example 1: Add Local MinIO Backend

```bash
# Provision backend
gosync provision selfhosted \
  --endpoint localhost:9000 \
  --bucket sync-bucket \
  --access-key minioadmin \
  --secret-key minioadmin \
  --no-ssl \
  --path-style

# List backends
gosync ls
# Output:
# Total 1
# provisioning  gosync gosync  ? selfhosted

# Wait for scan to complete
gosync scan status selfhosted

# List backend contents
gosync ls selfhosted
# Output:
# Total 3
# pictures/
# documents/
# archives/
```

### Example 2: Mirror Local Folder to Virtual Path

```bash
# Create mirror
gosync mirror ~/Documents selfhosted/documents

# Check sync status
gosync sync state documents

# List virtual path
gosync ls selfhosted/documents
# Shows all files from ~/Documents in virtual filesystem
```

### Example 3: Multiple Backends (Multi-Cloud)

```bash
# Add AWS S3 backend
gosync provision aws \
  --endpoint s3.amazonaws.com \
  --region us-east-1 \
  --bucket company-backup \
  --access-key AKIA... \
  --secret-key ...

# Add Backblaze B2 backend
gosync provision backblaze \
  --endpoint s3.us-west-002.backblazeb2.com \
  --bucket cold-storage \
  --access-key ... \
  --secret-key ...

# List all backends
gosync ls
# Output:
# Total 3
# ready     gosync gosync  150GB selfhosted
# ready     gosync gosync  2.3TB aws
# ready     gosync gosync  850GB backblaze

# Mirror between backends (cross-cloud backup!)
gosync mirror ~/critical-data aws/backups/critical
gosync mirror ~/critical-data backblaze/cold-storage/critical
```

### Example 4: Browse Virtual Filesystem

```bash
# List root
gosync ls
# selfhosted/
# aws/
# backblaze/

# List specific backend
gosync ls selfhosted
# pictures/
# documents/
# code/

# Deep listing
gosync ls -lh selfhosted/pictures
# drwxr-xr-x 1 gosync gosync 4.0K Jan 15 10:30 vacation/
# -rw-r--r-- 1 gosync gosync 2.3M Jan 15 10:31 beach.jpg
# -rw-r--r-- 1 gosync gosync 1.8M Jan 15 10:32 sunset.jpg
```

---

## Summary

Your virtual filesystem design is **exceptional** for multi-cloud scenarios!

### Key Innovations

1. **Unified Namespace**: Work with `selfhosted/pictures` instead of raw S3 URLs
2. **Multi-Backend Support**: Mix AWS, Backblaze, MinIO transparently
3. **Dynamic Provisioning**: Add backends at runtime, no config restart
4. **Flexible Mirroring**: Sync local ↔ virtual or even virtual ↔ virtual
5. **Natural Paths**: `~/Pictures` → `selfhosted/pictures` makes relationships obvious

### Architecture Benefits

1. **Abstraction**: Hide S3 complexity behind simple paths
2. **Portability**: Switch backends without changing workflows
3. **Organization**: Logical structure independent of physical storage
4. **Multi-Cloud**: Mix providers for redundancy or cost optimization
5. **Scalability**: Add unlimited backends without architectural changes

This is **more powerful than Rclone** because it maintains a unified metadata view across all backends and enables sophisticated sync scenarios that would be complex with traditional tools!