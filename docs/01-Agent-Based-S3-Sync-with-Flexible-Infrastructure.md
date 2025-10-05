# GoSync: Agent-Based S3 Sync with Flexible Infrastructure

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Agent-Based Design](#agent-based-design)
3. [Metadata Store Strategies](#metadata-store-strategies)
4. [Storage Engine Abstraction](#storage-engine-abstraction)
5. [Redis Cache Layer](#redis-cache-layer)
6. [CLI Command Structure](#cli-command-structure)
7. [Configuration System](#configuration-system)
8. [Implementation Guide](#implementation-guide)

---

## Architecture Overview

### Core Concept

GoSync is a **decentralized, agent-based sync client** where:
- **No centralized API server required**
- **Metadata database IS the coordination mechanism**
- **Agents connect directly to shared infrastructure** (PostgreSQL, Redis, S3)
- **CLI commands control and query running agents**
- **Flexible deployment options** from simple (SQLite-only) to advanced (PostgreSQL + Redis)

```
┌─────────────────────────────────────────────────────────┐
│                  Simple Deployment                      │
│                  (Single Machine)                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────────────────────────────────┐           │
│  │         GoSync Agent                     │           │
│  │  ┌────────────┐     ┌────────────┐       │           │
│  │  │  Watcher   │─────│   Engine   │       │           │
│  │  └────────────┘     └──────┬─────┘       │           │
│  │                            │             │           │
│  │              ┌─────────────▼─────┐       │           │
│  │              │  SQLite Metadata  │       │           │
│  │              │  (Local File)     │       │           │
│  │              └───────────────────┘       │           │
│  │                      │                   │           │
│  └──────────────────────┼───────────────────┘           │
│                         │                               │
│                         ▼                               │
│              ┌──────────────────┐                       │
│              │   S3 Storage     │                       │
│              └──────────────────┘                       │
│                                                         │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│              Advanced Deployment                        │
│              (Shared Infrastructure)                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌───────────────┐      ┌───────────────┐               │
│  │ Agent 1       │      │ Agent 2       │               │
│  │ (Laptop)      │      │ (Desktop)     │               │
│  └───────┬───────┘      └───────┬───────┘               │
│          │                      │                       │
│          └──────────┬───────────┘                       │
│                     ▼                                   │
│          ┌──────────────────────┐                       │
│          │  PostgreSQL          │                       │
│          │  (Shared Metadata)   │───────────┐           │
│          └──────────────────────┘           │           │
│                     │                       │           │
│                     ▼                       │           │
│          ┌──────────────────────┐           │           │
│          │  Redis               │           │           │
│          │  (Cache Layer)       │───────────┘           │
│          └──────────────────────┘                       │
│                     │                                   │
│                     ▼                                   │
│          ┌──────────────────────┐                       │
│          │  S3 Storage          │                       │
│          │  (MinIO/AWS)         │                       │
│          └──────────────────────┘                       │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Key Design Principles

1. **No Mandatory Server**: Everything can run client-side with SQLite
2. **Shared Coordination**: Multiple agents coordinate via shared PostgreSQL
3. **Optional Performance**: Redis adds caching without complexity
4. **Agent Architecture**: Like HashiCorp tools (Consul, Nomad)
5. **Files Stay Intact**: S3 files remain browsable and unmodified

---

## Agent-Based Design

### Agent vs CLI Split

```go
// cmd/gosync/main.go
package main

import (
    "fmt"
    "os"

    "github.com/mwantia/gosync/cmd/gosync/cli"
    "github.com/mwantia/gosync/cmd/gosync/cli/agent"
    "github.com/mwantia/gosync/cmd/gosync/cli/sync"
    "github.com/mwantia/gosync/cmd/gosync/cli/config"
)

func main() {
    root := cli.NewRootCommand(cli.VersionInfo{
        Version: version,
        Commit:  commit,
    })

    // Agent command - runs the daemon
    root.AddCommand(agent.NewAgentCommand())
    
    // Sync commands - interact with running agent
    root.AddCommand(sync.NewSyncCommand())
    
    // Config commands - manage configuration
    root.AddCommand(config.NewConfigCommand())
    
    // Version command
    root.AddCommand(cli.NewVersionCommand())

    if err := root.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
```

### Agent Command Structure

```go
// cmd/gosync/cli/agent/agent.go
package agent

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/spf13/cobra"
    "github.com/mwantia/gosync/internal/agent"
    "github.com/mwantia/gosync/internal/config"
)

func NewAgentCommand() *cobra.Command {
    var configPath string
    var dataDir string

    cmd := &cobra.Command{
        Use:   "agent",
        Short: "Run the GoSync agent daemon",
        Long: `Start the GoSync agent daemon that monitors and syncs files.
        
The agent runs in the background and handles:
- File system monitoring
- Synchronization with S3
- Metadata management
- Coordination with other agents (if using PostgreSQL)

Examples:
  # Run agent with config file
  gosync agent --config /etc/gosync/config.yaml
  
  # Run agent in foreground (development)
  gosync agent --config ./config.yaml
  
  # Run agent as systemd service
  systemctl start gosync`,
        
        RunE: func(cmd *cobra.Command, args []string) error {
            // Load configuration
            cfg, err := config.LoadConfig(configPath)
            if err != nil {
                return fmt.Errorf("failed to load config: %w", err)
            }

            // Override data directory if specified
            if dataDir != "" {
                cfg.DataDir = dataDir
            }

            // Create agent
            agent, err := agent.NewAgent(cfg)
            if err != nil {
                return fmt.Errorf("failed to create agent: %w", err)
            }

            // Setup signal handling
            ctx, cancel := signal.NotifyContext(
                context.Background(),
                os.Interrupt,
                syscall.SIGTERM,
            )
            defer cancel()

            // Start agent
            return agent.Run(ctx)
        },
    }

    cmd.Flags().StringVar(&configPath, "config", "", "path to config file")
    cmd.Flags().StringVar(&dataDir, "data-dir", "", "override data directory")
    cmd.MarkFlagRequired("config")

    return cmd
}
```

### Sync Management Commands

```go
// cmd/gosync/cli/sync/sync.go
package sync

import (
    "github.com/spf13/cobra"
)

func NewSyncCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "sync",
        Short: "Manage sync operations",
        Long:  `Commands to manage and monitor sync operations`,
    }

    // Add subcommands
    cmd.AddCommand(newStateCommand())
    cmd.AddCommand(newProvisionCommand())
    cmd.AddCommand(newMirrorCommand())
    cmd.AddCommand(newListCommand())
    cmd.AddCommand(newPauseCommand())
    cmd.AddCommand(newResumeCommand())

    return cmd
}

// gosync sync state - Check agent sync status
func newStateCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "state",
        Short: "Show current sync state",
        Long: `Display the current state of all active syncs.
        
Shows:
- Sync configurations
- Files in sync queue
- Current sync status
- Last sync time
- Error count`,
        
        RunE: func(cmd *cobra.Command, args []string) error {
            client, err := getAgentClient()
            if err != nil {
                return err
            }
            
            state, err := client.GetSyncState(cmd.Context())
            if err != nil {
                return err
            }
            
            printSyncState(state)
            return nil
        },
    }
}

// gosync sync provision <target> - Provision metadata from S3
func newProvisionCommand() *cobra.Command {
    var dryRun bool
    var recursive bool
    
    cmd := &cobra.Command{
        Use:   "provision <s3-path>",
        Short: "Provision metadata from S3 bucket",
        Long: `Scan an S3 bucket/path and populate the metadata database.
        
This is useful for:
- Initial setup with existing S3 data
- Recovering from metadata loss
- Adding new buckets to sync

The provision command will:
1. List all files in the specified S3 path
2. Extract metadata (size, modified time, etc.)
3. Populate the metadata database
4. NOT download files (use sync for that)

Examples:
  # Provision entire bucket
  gosync sync provision /
  
  # Provision specific path
  gosync sync provision /documents
  
  # Dry run (show what would be provisioned)
  gosync sync provision / --dry-run
  
  # Provision recursively (default)
  gosync sync provision /documents --recursive`,
        
        Args: cobra.ExactArgs(1),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            s3Path := args[0]
            
            client, err := getAgentClient()
            if err != nil {
                return err
            }
            
            opts := ProvisionOptions{
                S3Path:    s3Path,
                DryRun:    dryRun,
                Recursive: recursive,
            }
            
            return client.Provision(cmd.Context(), opts)
        },
    }
    
    cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be provisioned")
    cmd.Flags().BoolVar(&recursive, "recursive", true, "provision recursively")
    
    return cmd
}

// gosync sync mirror <local> <remote> - Create new sync mirror
func newMirrorCommand() *cobra.Command {
    var bidirectional bool
    var ignorePatterns []string
    var syncName string
    
    cmd := &cobra.Command{
        Use:   "mirror <local-path> <s3-path>",
        Short: "Create a new sync mirror",
        Long: `Create a new sync relationship between a local path and S3 path.
        
The mirror command sets up bidirectional sync between:
- Local directory (e.g., ~/Documents)
- S3 path (e.g., /backups/documents)

Examples:
  # Create bidirectional mirror
  gosync sync mirror ~/Documents /backups/documents
  
  # Create with custom name
  gosync sync mirror ~/Pictures /photos --name my-photos
  
  # Add ignore patterns
  gosync sync mirror ~/Code /code --ignore "*.log,node_modules,target"
  
  # Upload-only (no download)
  gosync sync mirror ~/Backups /backups --no-bidirectional`,
        
        Args: cobra.ExactArgs(2),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            localPath := args[0]
            s3Path := args[1]
            
            client, err := getAgentClient()
            if err != nil {
                return err
            }
            
            // Expand home directory
            localPath = expandPath(localPath)
            
            // Validate paths
            if err := validateLocalPath(localPath); err != nil {
                return err
            }
            if err := validateS3Path(s3Path); err != nil {
                return err
            }
            
            opts := MirrorOptions{
                Name:          syncName,
                LocalPath:     localPath,
                S3Path:        s3Path,
                Bidirectional: bidirectional,
                IgnorePatterns: ignorePatterns,
            }
            
            return client.CreateMirror(cmd.Context(), opts)
        },
    }
    
    cmd.Flags().BoolVar(&bidirectional, "bidirectional", true, "enable bidirectional sync")
    cmd.Flags().StringSliceVar(&ignorePatterns, "ignore", nil, "patterns to ignore")
    cmd.Flags().StringVar(&syncName, "name", "", "custom name for sync")
    
    return cmd
}

// gosync sync list - List all configured syncs
func newListCommand() *cobra.Command {
    var format string
    
    cmd := &cobra.Command{
        Use:   "list",
        Short: "List all configured syncs",
        Long: `Display all configured sync relationships.
        
Shows:
- Sync name
- Local path
- S3 path
- Status (active, paused, error)
- Last sync time
- File count

Examples:
  # List as table (default)
  gosync sync list
  
  # List as JSON
  gosync sync list --format json
  
  # List as YAML
  gosync sync list --format yaml`,
        
        RunE: func(cmd *cobra.Command, args []string) error {
            client, err := getAgentClient()
            if err != nil {
                return err
            }
            
            syncs, err := client.ListSyncs(cmd.Context())
            if err != nil {
                return err
            }
            
            switch format {
            case "json":
                printJSON(syncs)
            case "yaml":
                printYAML(syncs)
            default:
                printTable(syncs)
            }
            
            return nil
        },
    }
    
    cmd.Flags().StringVar(&format, "format", "table", "output format (table, json, yaml)")
    
    return cmd
}

// gosync sync pause <name> - Pause a sync
func newPauseCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "pause <sync-name>",
        Short: "Pause a sync",
        Args:  cobra.ExactArgs(1),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            syncName := args[0]
            
            client, err := getAgentClient()
            if err != nil {
                return err
            }
            
            return client.PauseSync(cmd.Context(), syncName)
        },
    }
}

// gosync sync resume <name> - Resume a paused sync
func newResumeCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "resume <sync-name>",
        Short: "Resume a paused sync",
        Args:  cobra.ExactArgs(1),
        
        RunE: func(cmd *cobra.Command, args []string) error {
            syncName := args[0]
            
            client, err := getAgentClient()
            if err != nil {
                return err
            }
            
            return client.ResumeSync(cmd.Context(), syncName)
        },
    }
}
```

### Agent RPC Interface

```go
// internal/agent/rpc.go
package agent

import (
    "context"
    "encoding/json"
    "net"
    "net/http"
    "os"
    "path/filepath"
)

// Agent exposes an HTTP API for CLI commands
type AgentRPC struct {
    agent  *Agent
    socket string
}

func NewAgentRPC(agent *Agent, dataDir string) *AgentRPC {
    socket := filepath.Join(dataDir, "gosync.sock")
    
    return &AgentRPC{
        agent:  agent,
        socket: socket,
    }
}

func (rpc *AgentRPC) Start(ctx context.Context) error {
    // Remove old socket if exists
    os.Remove(rpc.socket)
    
    // Create Unix domain socket
    listener, err := net.Listen("unix", rpc.socket)
    if err != nil {
        return err
    }
    
    // Setup HTTP handlers
    mux := http.NewServeMux()
    mux.HandleFunc("/sync/state", rpc.handleSyncState)
    mux.HandleFunc("/sync/provision", rpc.handleProvision)
    mux.HandleFunc("/sync/mirror", rpc.handleMirror)
    mux.HandleFunc("/sync/list", rpc.handleList)
    mux.HandleFunc("/sync/pause", rpc.handlePause)
    mux.HandleFunc("/sync/resume", rpc.handleResume)
    
    server := &http.Server{Handler: mux}
    
    // Start server
    go func() {
        <-ctx.Done()
        server.Close()
        listener.Close()
        os.Remove(rpc.socket)
    }()
    
    return server.Serve(listener)
}

func (rpc *AgentRPC) handleSyncState(w http.ResponseWriter, r *http.Request) {
    state := rpc.agent.GetSyncState()
    
    json.NewEncoder(w).Encode(state)
}

func (rpc *AgentRPC) handleProvision(w http.ResponseWriter, r *http.Request) {
    var opts ProvisionOptions
    if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    if err := rpc.agent.Provision(r.Context(), opts); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}

func (rpc *AgentRPC) handleMirror(w http.ResponseWriter, r *http.Request) {
    var opts MirrorOptions
    if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    if err := rpc.agent.CreateMirror(r.Context(), opts); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}

// ... other handlers
```

### CLI Client

```go
// internal/client/agent_client.go
package client

import (
    "context"
    "encoding/json"
    "fmt"
    "net"
    "net/http"
)

type AgentClient struct {
    socket string
    client *http.Client
}

func NewAgentClient(dataDir string) *AgentClient {
    socket := filepath.Join(dataDir, "gosync.sock")
    
    return &AgentClient{
        socket: socket,
        client: &http.Client{
            Transport: &http.Transport{
                DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
                    return net.Dial("unix", socket)
                },
            },
        },
    }
}

func (c *AgentClient) GetSyncState(ctx context.Context) (*SyncState, error) {
    resp, err := c.client.Get("http://unix/sync/state")
    if err != nil {
        return nil, fmt.Errorf("failed to connect to agent: %w", err)
    }
    defer resp.Body.Close()
    
    var state SyncState
    if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
        return nil, err
    }
    
    return &state, nil
}

func (c *AgentClient) Provision(ctx context.Context, opts ProvisionOptions) error {
    data, err := json.Marshal(opts)
    if err != nil {
        return err
    }
    
    resp, err := c.client.Post("http://unix/sync/provision", "application/json", bytes.NewReader(data))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("provision failed: %s", body)
    }
    
    return nil
}

// ... other methods
```

---

## Metadata Store Strategies

### Storage Engine Interface

```go
// pkg/metadata/store.go
package metadata

import (
    "context"
    "time"
)

// MetadataStore is the core interface for metadata operations
type MetadataStore interface {
    // File operations
    GetFile(ctx context.Context, id int64) (*File, error)
    GetFileByPath(ctx context.Context, path string) (*File, error)
    ListFiles(ctx context.Context, opts ListOptions) ([]*File, error)
    UpsertFile(ctx context.Context, file *File) error
    DeleteFile(ctx context.Context, id int64) error
    
    // Change tracking
    GetChangesSince(ctx context.Context, cursor int64, limit int) ([]*FileChange, error)
    RecordChange(ctx context.Context, change *FileChange) error
    
    // Sync state
    GetSyncState(ctx context.Context, clientID string) (*SyncState, error)
    UpdateSyncState(ctx context.Context, state *SyncState) error
    
    // Sync configuration
    GetSyncConfig(ctx context.Context, name string) (*SyncConfig, error)
    ListSyncConfigs(ctx context.Context) ([]*SyncConfig, error)
    UpsertSyncConfig(ctx context.Context, config *SyncConfig) error
    DeleteSyncConfig(ctx context.Context, name string) error
    
    // Lifecycle
    Close() error
}

type File struct {
    ID           int64
    Path         string
    S3Key        string
    Size         int64
    ModifiedTime time.Time
    CreatedTime  time.Time
    ETag         string
    Checksum     string
    MimeType     string
    IsDirectory  bool
    ParentID     *int64
    Version      int
    IsDeleted    bool
    DeletedAt    *time.Time
}

type FileChange struct {
    ID         int64
    FileID     int64
    ChangeType string // CREATE, UPDATE, DELETE, RENAME
    ChangedAt  time.Time
    ClientID   string
    OldPath    *string
    NewPath    *string
}

type SyncState struct {
    ClientID     string
    LastSyncTime time.Time
    SyncCursor   int64
    DeviceName   string
    Platform     string
}

type SyncConfig struct {
    Name          string
    LocalPath     string
    S3Path        string
    Bidirectional bool
    IgnorePatterns []string
    Status        string // active, paused, error
    LastSyncTime  *time.Time
    FileCount     int64
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### SQLite Implementation (Simple)

```go
// pkg/metadata/sqlite/store.go
package sqlite

import (
    "context"
    "database/sql"
    "fmt"
    
    _ "github.com/mattn/go-sqlite3"
    "github.com/mwantia/gosync/pkg/metadata"
)

type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
    if err != nil {
        return nil, err
    }
    
    store := &SQLiteStore{db: db}
    
    if err := store.initialize(); err != nil {
        db.Close()
        return nil, err
    }
    
    return store, nil
}

func (s *SQLiteStore) initialize() error {
    schema := `
    CREATE TABLE IF NOT EXISTS files (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        path TEXT NOT NULL,
        s3_key TEXT NOT NULL,
        size INTEGER NOT NULL,
        modified_time DATETIME NOT NULL,
        created_time DATETIME NOT NULL,
        etag TEXT,
        checksum TEXT,
        mime_type TEXT,
        is_directory BOOLEAN DEFAULT 0,
        parent_id INTEGER,
        version INTEGER DEFAULT 1,
        is_deleted BOOLEAN DEFAULT 0,
        deleted_at DATETIME,
        UNIQUE(path),
        FOREIGN KEY(parent_id) REFERENCES files(id)
    );
    
    CREATE INDEX IF NOT EXISTS idx_files_parent ON files(parent_id);
    CREATE INDEX IF NOT EXISTS idx_files_s3_key ON files(s3_key);
    CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(is_deleted);
    
    CREATE TABLE IF NOT EXISTS file_changes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        file_id INTEGER NOT NULL,
        change_type TEXT NOT NULL,
        changed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
        client_id TEXT,
        old_path TEXT,
        new_path TEXT,
        FOREIGN KEY(file_id) REFERENCES files(id)
    );
    
    CREATE INDEX IF NOT EXISTS idx_changes_time ON file_changes(changed_at);
    
    CREATE TABLE IF NOT EXISTS sync_states (
        client_id TEXT PRIMARY KEY,
        last_sync_time DATETIME NOT NULL,
        sync_cursor INTEGER,
        device_name TEXT,
        platform TEXT
    );
    
    CREATE TABLE IF NOT EXISTS sync_configs (
        name TEXT PRIMARY KEY,
        local_path TEXT NOT NULL,
        s3_path TEXT NOT NULL,
        bidirectional BOOLEAN DEFAULT 1,
        ignore_patterns TEXT,
        status TEXT DEFAULT 'active',
        last_sync_time DATETIME,
        file_count INTEGER DEFAULT 0,
        created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    `
    
    _, err := s.db.Exec(schema)
    return err
}

func (s *SQLiteStore) GetFileByPath(ctx context.Context, path string) (*metadata.File, error) {
    var file metadata.File
    
    err := s.db.QueryRowContext(ctx, `
        SELECT id, path, s3_key, size, modified_time, created_time, 
               etag, checksum, mime_type, is_directory, parent_id, version
        FROM files
        WHERE path = ? AND is_deleted = 0
    `, path).Scan(
        &file.ID, &file.Path, &file.S3Key, &file.Size,
        &file.ModifiedTime, &file.CreatedTime, &file.ETag,
        &file.Checksum, &file.MimeType, &file.IsDirectory,
        &file.ParentID, &file.Version,
    )
    
    if err == sql.ErrNoRows {
        return nil, metadata.ErrNotFound
    }
    
    return &file, err
}

// ... implement other MetadataStore methods
```

### PostgreSQL Implementation (Shared)

```go
// pkg/metadata/postgres/store.go
package postgres

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    _ "github.com/lib/pq"
    "github.com/mwantia/gosync/pkg/metadata"
)

type PostgresStore struct {
    db *sql.DB
}

func NewPostgresStore(connString string) (*PostgresStore, error) {
    db, err := sql.Open("postgres", connString)
    if err != nil {
        return nil, err
    }
    
    // Connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    store := &PostgresStore{db: db}
    
    if err := store.initialize(); err != nil {
        db.Close()
        return nil, err
    }
    
    return store, nil
}

func (s *PostgresStore) initialize() error {
    schema := `
    CREATE TABLE IF NOT EXISTS files (
        id BIGSERIAL PRIMARY KEY,
        path TEXT NOT NULL,
        s3_key TEXT NOT NULL,
        size BIGINT NOT NULL,
        modified_time TIMESTAMP NOT NULL,
        created_time TIMESTAMP NOT NULL,
        etag VARCHAR(64),
        checksum VARCHAR(64),
        mime_type VARCHAR(255),
        is_directory BOOLEAN DEFAULT FALSE,
        parent_id BIGINT REFERENCES files(id),
        version INTEGER DEFAULT 1,
        is_deleted BOOLEAN DEFAULT FALSE,
        deleted_at TIMESTAMP,
        UNIQUE(path)
    );
    
    CREATE INDEX IF NOT EXISTS idx_files_parent ON files(parent_id);
    CREATE INDEX IF NOT EXISTS idx_files_s3_key ON files(s3_key);
    CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(is_deleted);
    CREATE INDEX IF NOT EXISTS idx_files_modified ON files(modified_time);
    
    CREATE TABLE IF NOT EXISTS file_changes (
        id BIGSERIAL PRIMARY KEY,
        file_id BIGINT NOT NULL REFERENCES files(id),
        change_type VARCHAR(20) NOT NULL CHECK (change_type IN ('CREATE', 'UPDATE', 'DELETE', 'RENAME')),
        changed_at TIMESTAMP NOT NULL DEFAULT NOW(),
        client_id UUID,
        old_path TEXT,
        new_path TEXT
    );
    
    CREATE INDEX IF NOT EXISTS idx_changes_time ON file_changes(changed_at);
    CREATE INDEX IF NOT EXISTS idx_changes_file ON file_changes(file_id);
    
    CREATE TABLE IF NOT EXISTS sync_states (
        client_id UUID PRIMARY KEY,
        last_sync_time TIMESTAMP NOT NULL,
        sync_cursor BIGINT,
        device_name TEXT,
        platform TEXT
    );
    
    CREATE TABLE IF NOT EXISTS sync_configs (
        name TEXT PRIMARY KEY,
        local_path TEXT NOT NULL,
        s3_path TEXT NOT NULL,
        bidirectional BOOLEAN DEFAULT TRUE,
        ignore_patterns TEXT[],
        status TEXT DEFAULT 'active' CHECK (status IN ('active', 'paused', 'error')),
        last_sync_time TIMESTAMP,
        file_count BIGINT DEFAULT 0,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMP NOT NULL DEFAULT NOW()
    );
    `
    
    _, err := s.db.Exec(schema)
    return err
}

func (s *PostgresStore) GetFileByPath(ctx context.Context, path string) (*metadata.File, error) {
    var file metadata.File
    
    err := s.db.QueryRowContext(ctx, `
        SELECT id, path, s3_key, size, modified_time, created_time, 
               etag, checksum, mime_type, is_directory, parent_id, version
        FROM files
        WHERE path = $1 AND is_deleted = FALSE
    `, path).Scan(
        &file.ID, &file.Path, &file.S3Key, &file.Size,
        &file.ModifiedTime, &file.CreatedTime, &file.ETag,
        &file.Checksum, &file.MimeType, &file.IsDirectory,
        &file.ParentID, &file.Version,
    )
    
    if err == sql.ErrNoRows {
        return nil, metadata.ErrNotFound
    }
    
    return &file, err
}

// PostgreSQL excels at concurrent operations
func (s *PostgresStore) BatchUpsertFiles(ctx context.Context, files []*metadata.File) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO files (path, s3_key, size, modified_time, created_time, etag, checksum, mime_type, is_directory)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (path) 
        DO UPDATE SET 
            size = EXCLUDED.size,
            modified_time = EXCLUDED.modified_time,
            etag = EXCLUDED.etag,
            checksum = EXCLUDED.checksum,
            version = files.version + 1
        RETURNING id
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, file := range files {
        err = stmt.QueryRowContext(ctx,
            file.Path, file.S3Key, file.Size, file.ModifiedTime,
            file.CreatedTime, file.ETag, file.Checksum, file.MimeType,
            file.IsDirectory,
        ).Scan(&file.ID)
        
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// ... implement other MetadataStore methods
```

### Factory Pattern

```go
// pkg/metadata/factory.go
package metadata

import (
    "fmt"
    
    "github.com/mwantia/gosync/internal/config"
    "github.com/mwantia/gosync/pkg/metadata/sqlite"
    "github.com/mwantia/gosync/pkg/metadata/postgres"
)

func NewMetadataStore(cfg config.MetadataConfig) (MetadataStore, error) {
    switch cfg.Type {
    case "sqlite":
        return sqlite.NewSQLiteStore(cfg.SQLite.Path)
        
    case "postgres":
        connStr := fmt.Sprintf(
            "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
            cfg.Postgres.Host,
            cfg.Postgres.Port,
            cfg.Postgres.User,
            cfg.Postgres.Password,
            cfg.Postgres.Database,
            cfg.Postgres.SSLMode,
        )
        return postgres.NewPostgresStore(connStr)
        
    default:
        return nil, fmt.Errorf("unsupported metadata store type: %s", cfg.Type)
    }
}
```

---

## Storage Engine Abstraction

### S3 Interface

```go
// pkg/storage/interface.go
package storage

import (
    "context"
    "io"
    "time"
)

type StorageEngine interface {
    // Upload operations
    Upload(ctx context.Context, key string, reader io.Reader, size int64) error
    UploadMultipart(ctx context.Context, key string, reader io.Reader, size int64) error
    
    // Download operations
    Download(ctx context.Context, key string, writer io.Writer) error
    DownloadRange(ctx context.Context, key string, writer io.Writer, start, end int64) error
    
    // Metadata operations
    Head(ctx context.Context, key string) (*ObjectInfo, error)
    List(ctx context.Context, prefix string) ([]*ObjectInfo, error)
    
    // Delete operations
    Delete(ctx context.Context, key string) error
    DeleteMultiple(ctx context.Context, keys []string) error
}

type ObjectInfo struct {
    Key          string
    Size         int64
    ETag         string
    LastModified time.Time
    ContentType  string
}
```

### MinIO/S3 Implementation

```go
// pkg/storage/s3/s3.go
package s3

import (
    "context"
    "io"
    
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
    "github.com/aws/aws-sdk-go/service/s3/s3manager"
    
    "github.com/mwantia/gosync/internal/config"
    "github.com/mwantia/gosync/pkg/storage"
)

type S3Storage struct {
    client   *s3.S3
    uploader *s3manager.Uploader
    bucket   string
}

func NewS3Storage(cfg config.S3Config) (*S3Storage, error) {
    awsCfg := &aws.Config{
        Endpoint:         aws.String(cfg.Endpoint),
        Region:           aws.String(cfg.Region),
        Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
        S3ForcePathStyle: aws.Bool(true), // For MinIO
    }
    
    sess, err := session.NewSession(awsCfg)
    if err != nil {
        return nil, err
    }
    
    return &S3Storage{
        client:   s3.New(sess),
        uploader: s3manager.NewUploader(sess),
        bucket:   cfg.Bucket,
    }, nil
}

func (s *S3Storage) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
    _, err := s.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
        Bucket: aws.String(s.bucket),
        Key:    aws.String(key),
        Body:   reader,
    })
    return err
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]*storage.ObjectInfo, error) {
    var objects []*storage.ObjectInfo
    
    err := s.client.ListObjectsV2PagesWithContext(ctx,
        &s3.ListObjectsV2Input{
            Bucket: aws.String(s.bucket),
            Prefix: aws.String(prefix),
        },
        func(page *s3.ListObjectsV2Output, lastPage bool) bool {
            for _, obj := range page.Contents {
                objects = append(objects, &storage.ObjectInfo{
                    Key:          *obj.Key,
                    Size:         *obj.Size,
                    ETag:         *obj.ETag,
                    LastModified: *obj.LastModified,
                })
            }
            return true
        },
    )
    
    return objects, err
}

// ... implement other StorageEngine methods
```

---

## Redis Cache Layer

### Cache Interface

```go
// pkg/cache/interface.go
package cache

import (
    "context"
    "time"
)

type CacheLayer interface {
    // Metadata caching
    GetMetadata(ctx context.Context, key string) ([]byte, error)
    SetMetadata(ctx context.Context, key string, data []byte, ttl time.Duration) error
    DeleteMetadata(ctx context.Context, key string) error
    
    // File content caching (for small files)
    GetFile(ctx context.Context, key string) ([]byte, error)
    SetFile(ctx context.Context, key string, data []byte, ttl time.Duration) error
    DeleteFile(ctx context.Context, key string) error
    
    // List caching
    GetList(ctx context.Context, key string) ([]string, error)
    SetList(ctx context.Context, key string, items []string, ttl time.Duration) error
    
    // Batch operations
    GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error)
    SetMultiple(ctx context.Context, items map[string][]byte, ttl time.Duration) error
    
    // Cache management
    Flush(ctx context.Context) error
    Stats(ctx context.Context) (*CacheStats, error)
}

type CacheStats struct {
    Hits          int64
    Misses        int64
    Keys          int64
    MemoryUsed    int64
    MemoryMax     int64
}
```

### Redis Implementation

```go
// pkg/cache/redis/redis.go
package redis

import (
    "context"
    "fmt"
    "time"
    
    "github.com/redis/go-redis/v9"
    
    "github.com/mwantia/gosync/internal/config"
    "github.com/mwantia/gosync/pkg/cache"
)

type RedisCache struct {
    client *redis.Client
    prefix string
}

func NewRedisCache(cfg config.RedisConfig) (*RedisCache, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
        Password:     cfg.Password,
        DB:           cfg.DB,
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
    })
    
    // Test connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to redis: %w", err)
    }
    
    return &RedisCache{
        client: client,
        prefix: cfg.Prefix,
    }, nil
}

func (r *RedisCache) GetMetadata(ctx context.Context, key string) ([]byte, error) {
    fullKey := r.prefix + ":meta:" + key
    
    data, err := r.client.Get(ctx, fullKey).Bytes()
    if err == redis.Nil {
        return nil, cache.ErrNotFound
    }
    return data, err
}

func (r *RedisCache) SetMetadata(ctx context.Context, key string, data []byte, ttl time.Duration) error {
    fullKey := r.prefix + ":meta:" + key
    return r.client.Set(ctx, fullKey, data, ttl).Err()
}

func (r *RedisCache) GetFile(ctx context.Context, key string) ([]byte, error) {
    fullKey := r.prefix + ":file:" + key
    
    data, err := r.client.Get(ctx, fullKey).Bytes()
    if err == redis.Nil {
        return nil, cache.ErrNotFound
    }
    return data, err
}

func (r *RedisCache) SetFile(ctx context.Context, key string, data []byte, ttl time.Duration) error {
    fullKey := r.prefix + ":file:" + key
    
    // Only cache files smaller than 1MB
    if len(data) > 1024*1024 {
        return cache.ErrTooLarge
    }
    
    return r.client.Set(ctx, fullKey, data, ttl).Err()
}

func (r *RedisCache) GetMultiple(ctx context.Context, keys []string) (map[string][]byte, error) {
    fullKeys := make([]string, len(keys))
    for i, key := range keys {
        fullKeys[i] = r.prefix + ":meta:" + key
    }
    
    values, err := r.client.MGet(ctx, fullKeys...).Result()
    if err != nil {
        return nil, err
    }
    
    result := make(map[string][]byte)
    for i, val := range values {
        if val != nil {
            if data, ok := val.(string); ok {
                result[keys[i]] = []byte(data)
            }
        }
    }
    
    return result, nil
}

func (r *RedisCache) Stats(ctx context.Context) (*cache.CacheStats, error) {
    info, err := r.client.Info(ctx, "stats", "memory").Result()
    if err != nil {
        return nil, err
    }
    
    // Parse Redis INFO output
    stats := &cache.CacheStats{}
    // ... parse info string
    
    return stats, nil
}

// ... implement other CacheLayer methods
```

### Cache-Through Metadata Store

```go
// pkg/metadata/cached/store.go
package cached

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/mwantia/gosync/pkg/cache"
    "github.com/mwantia/gosync/pkg/metadata"
)

// CachedStore wraps a metadata store with Redis caching
type CachedStore struct {
    store metadata.MetadataStore
    cache cache.CacheLayer
    ttl   time.Duration
}

func NewCachedStore(store metadata.MetadataStore, cache cache.CacheLayer, ttl time.Duration) *CachedStore {
    return &CachedStore{
        store: store,
        cache: cache,
        ttl:   ttl,
    }
}

func (c *CachedStore) GetFileByPath(ctx context.Context, path string) (*metadata.File, error) {
    // Try cache first
    cacheKey := "file:path:" + path
    
    if data, err := c.cache.GetMetadata(ctx, cacheKey); err == nil {
        var file metadata.File
        if err := json.Unmarshal(data, &file); err == nil {
            return &file, nil
        }
    }
    
    // Cache miss, fetch from store
    file, err := c.store.GetFileByPath(ctx, path)
    if err != nil {
        return nil, err
    }
    
    // Update cache
    if data, err := json.Marshal(file); err == nil {
        c.cache.SetMetadata(ctx, cacheKey, data, c.ttl)
    }
    
    return file, nil
}

func (c *CachedStore) UpsertFile(ctx context.Context, file *metadata.File) error {
    // Update database
    if err := c.store.UpsertFile(ctx, file); err != nil {
        return err
    }
    
    // Invalidate cache
    cacheKey := "file:path:" + file.Path
    c.cache.DeleteMetadata(ctx, cacheKey)
    
    return nil
}

// ... wrap other MetadataStore methods with caching
```

---

## CLI Command Structure

### Complete Command Tree

```
gosync
├── agent                    # Run agent daemon
│   └── --config <path>     # Config file path
│
├── sync                     # Sync management
│   ├── state               # Show current sync state
│   ├── provision <path>    # Provision metadata from S3
│   │   ├── --dry-run       # Show what would be provisioned
│   │   └── --recursive     # Provision recursively
│   ├── mirror <local> <remote>  # Create new sync mirror
│   │   ├── --name <name>   # Custom sync name
│   │   ├── --ignore <patterns>  # Ignore patterns
│   │   └── --no-bidirectional   # Upload only
│   ├── list                # List all syncs
│   │   └── --format <fmt>  # Output format (table, json, yaml)
│   ├── pause <name>        # Pause a sync
│   ├── resume <name>       # Resume a sync
│   └── remove <name>       # Remove a sync
│
├── config                   # Configuration management
│   ├── init                # Initialize new config
│   ├── validate            # Validate config file
│   ├── show                # Show current config
│   └── generate            # Generate example configs
│
└── version                  # Show version info
    ├── --short             # Short version
    └── --json              # JSON output
```

---

## Configuration System

### Full Configuration Example

```yaml
# config.yaml - GoSync Configuration

# Data directory for agent state
data_dir: /var/lib/gosync

# Logging configuration
log:
  level: info              # debug, info, warn, error
  format: text            # text, json
  file: /var/log/gosync/gosync.log
  rotation:
    max_size: 100         # MB
    max_backups: 5
    max_age: 30           # days
    compress: true

# Metadata store configuration
metadata:
  type: postgres           # sqlite, postgres
  
  # SQLite configuration (when type: sqlite)
  sqlite:
    path: ${data_dir}/gosync.db
  
  # PostgreSQL configuration (when type: postgres)
  postgres:
    host: localhost
    port: 5432
    user: gosync
    password: ${POSTGRES_PASSWORD}  # Support env vars
    database: gosync
    sslmode: require

# Optional Redis cache
cache:
  enabled: true
  redis:
    host: localhost
    port: 6379
    password: ${REDIS_PASSWORD}
    db: 0
    prefix: gosync
  ttl: 5m                  # Cache TTL for metadata

# S3/MinIO configuration
s3:
  endpoint: s3.amazonaws.com  # or MinIO endpoint
  region: us-east-1
  access_key: ${S3_ACCESS_KEY}
  secret_key: ${S3_SECRET_KEY}
  bucket: my-sync-bucket
  use_ssl: true

# Sync configurations
syncs:
  - name: documents
    local_path: ~/Documents
    s3_path: /documents
    bidirectional: true
    ignore_patterns:
      - "*.tmp"
      - ".DS_Store"
      - "node_modules"
  
  - name: photos
    local_path: ~/Pictures
    s3_path: /photos
    bidirectional: true
    ignore_patterns:
      - "*.thumb"

# Sync engine settings
sync:
  interval: 60s            # How often to check for remote changes
  workers: 4               # Concurrent upload/download workers
  chunk_size: 5MB          # Multipart upload chunk size
  max_retries: 3

# Agent settings
agent:
  socket_path: ${data_dir}/gosync.sock
```

### Configuration Loading

```go
// internal/config/config.go
package config

import (
    "fmt"
    "os"
    "strings"
    "time"
    
    "github.com/spf13/viper"
)

type Config struct {
    DataDir  string          `mapstructure:"data_dir"`
    Log      LogConfig       `mapstructure:"log"`
    Metadata MetadataConfig  `mapstructure:"metadata"`
    Cache    CacheConfig     `mapstructure:"cache"`
    S3       S3Config        `mapstructure:"s3"`
    Syncs    []SyncConfig    `mapstructure:"syncs"`
    Sync     SyncSettings    `mapstructure:"sync"`
    Agent    AgentConfig     `mapstructure:"agent"`
}

type MetadataConfig struct {
    Type     string          `mapstructure:"type"`
    SQLite   SQLiteConfig    `mapstructure:"sqlite"`
    Postgres PostgresConfig  `mapstructure:"postgres"`
}

type SQLiteConfig struct {
    Path string `mapstructure:"path"`
}

type PostgresConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    User     string `mapstructure:"user"`
    Password string `mapstructure:"password"`
    Database string `mapstructure:"database"`
    SSLMode  string `mapstructure:"sslmode"`
}

type CacheConfig struct {
    Enabled bool        `mapstructure:"enabled"`
    Redis   RedisConfig `mapstructure:"redis"`
    TTL     time.Duration `mapstructure:"ttl"`
}

type RedisConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Password string `mapstructure:"password"`
    DB       int    `mapstructure:"db"`
    Prefix   string `mapstructure:"prefix"`
}

type S3Config struct {
    Endpoint  string `mapstructure:"endpoint"`
    Region    string `mapstructure:"region"`
    AccessKey string `mapstructure:"access_key"`
    SecretKey string `mapstructure:"secret_key"`
    Bucket    string `mapstructure:"bucket"`
    UseSSL    bool   `mapstructure:"use_ssl"`
}

type SyncConfig struct {
    Name           string   `mapstructure:"name"`
    LocalPath      string   `mapstructure:"local_path"`
    S3Path         string   `mapstructure:"s3_path"`
    Bidirectional  bool     `mapstructure:"bidirectional"`
    IgnorePatterns []string `mapstructure:"ignore_patterns"`
}

type SyncSettings struct {
    Interval   time.Duration `mapstructure:"interval"`
    Workers    int           `mapstructure:"workers"`
    ChunkSize  string        `mapstructure:"chunk_size"`
    MaxRetries int           `mapstructure:"max_retries"`
}

type AgentConfig struct {
    SocketPath string `mapstructure:"socket_path"`
}

func LoadConfig(path string) (*Config, error) {
    v := viper.New()
    
    // Set config file
    v.SetConfigFile(path)
    
    // Environment variable support
    v.SetEnvPrefix("GOSYNC")
    v.AutomaticEnv()
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    
    // Read config
    if err := v.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }
    
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }
    
    // Expand environment variables in strings
    cfg = expandEnvVars(cfg)
    
    // Validate config
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }
    
    return &cfg, nil
}

func (c *Config) Validate() error {
    if c.DataDir == "" {
        return fmt.Errorf("data_dir is required")
    }
    
    if c.Metadata.Type != "sqlite" && c.Metadata.Type != "postgres" {
        return fmt.Errorf("metadata.type must be 'sqlite' or 'postgres'")
    }
    
    if c.Metadata.Type == "sqlite" && c.Metadata.SQLite.Path == "" {
        return fmt.Errorf("metadata.sqlite.path is required when using sqlite")
    }
    
    if c.Metadata.Type == "postgres" {
        if c.Metadata.Postgres.Host == "" {
            return fmt.Errorf("metadata.postgres.host is required")
        }
    }
    
    if c.S3.Bucket == "" {
        return fmt.Errorf("s3.bucket is required")
    }
    
    return nil
}

func expandEnvVars(cfg Config) Config {
    // Expand ${VAR} and $VAR in string fields
    // Implementation left as exercise
    return cfg
}
```

---

## Implementation Guide

### Phase 1: Core Agent Infrastructure (Week 1-2)

**Goals:**
- Agent daemon architecture
- CLI command structure
- RPC interface via Unix socket
- Basic configuration system

**Tasks:**
1. Set up agent structure with fabric container
2. Implement Unix socket RPC server
3. Create CLI client
4. Build configuration loading
5. Add basic commands (state, list)

```go
// Example: Basic agent structure
type Agent struct {
    config   *config.Config
    metadata metadata.MetadataStore
    storage  storage.StorageEngine
    cache    cache.CacheLayer
    rpc      *AgentRPC
    syncs    map[string]*SyncWorker
}

func (a *Agent) Run(ctx context.Context) error {
    // Start RPC server
    go a.rpc.Start(ctx)
    
    // Load sync configurations
    for _, syncCfg := range a.config.Syncs {
        worker := NewSyncWorker(syncCfg, a.metadata, a.storage)
        a.syncs[syncCfg.Name] = worker
        go worker.Run(ctx)
    }
    
    // Wait for shutdown
    <-ctx.Done()
    return nil
}
```

### Phase 2: Metadata Store Implementation (Week 3)

**Goals:**
- SQLite implementation
- PostgreSQL implementation  
- Factory pattern
- Migration system

**Tasks:**
1. Define metadata store interface
2. Implement SQLite store
3. Implement PostgreSQL store
4. Create database migrations
5. Add tests for both implementations

### Phase 3: Sync Engine (Week 4-5)

**Goals:**
- File watcher
- Sync algorithm
- Upload/download workers
- Conflict resolution

**Tasks:**
1. Implement file watcher with fsnotify
2. Build sync engine with worker pool
3. Add change tracking
4. Implement conflict resolution
5. Add provision command

### Phase 4: Redis Cache Layer (Week 6)

**Goals:**
- Redis client
- Cache-through wrapper
- Cache invalidation
- Performance optimization

**Tasks:**
1. Implement Redis cache
2. Create cached metadata store wrapper
3. Add cache statistics
4. Optimize hit rates

### Phase 5: CLI Commands (Week 7)

**Goals:**
- Mirror command
- Provision command
- Management commands
- Output formatting

**Tasks:**
1. Implement mirror command
2. Implement provision command
3. Add pause/resume commands
4. Add formatted output (table, JSON, YAML)

### Phase 6: Production Ready (Week 8+)

**Goals:**
- Logging
- Metrics
- Error handling
- Documentation
- Testing

**Tasks:**
1. Add comprehensive logging
2. Implement metrics collection
3. Error handling and retry logic
4. Write documentation
5. Integration tests
6. Performance benchmarks

---

## Usage Examples

### Simple Deployment (SQLite Only)

```bash
# 1. Create config
cat > config.yaml <<EOF
data_dir: ~/.gosync
metadata:
  type: sqlite
  sqlite:
    path: ~/.gosync/metadata.db
s3:
  endpoint: s3.amazonaws.com
  region: us-east-1
  access_key: your-key
  secret_key: your-secret
  bucket: my-bucket
syncs:
  - name: documents
    local_path: ~/Documents
    s3_path: /documents
    bidirectional: true
EOF

# 2. Start agent
gosync agent --config config.yaml &

# 3. Check status
gosync sync state

# 4. Add new sync
gosync sync mirror ~/Pictures /photos --name photos

# 5. List syncs
gosync sync list
```

### Advanced Deployment (PostgreSQL + Redis)

```bash
# 1. Setup infrastructure
docker-compose up -d postgres redis minio

# 2. Create config
cat > config.yaml <<EOF
data_dir: /var/lib/gosync
metadata:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    user: gosync
    password: ${POSTGRES_PASSWORD}
    database: gosync
cache:
  enabled: true
  redis:
    host: localhost
    port: 6379
s3:
  endpoint: localhost:9000
  region: us-east-1
  access_key: minioadmin
  secret_key: minioadmin
  bucket: sync
  use_ssl: false
syncs:
  - name: shared-data
    local_path: /data/shared
    s3_path: /shared
    bidirectional: true
EOF

# 3. Install as systemd service
sudo cp gosync /usr/local/bin/
sudo cat > /etc/systemd/system/gosync.service <<EOF
[Unit]
Description=GoSync Agent
After=network.target

[Service]
Type=simple
User=gosync
ExecStart=/usr/local/bin/gosync agent --config /etc/gosync/config.yaml
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# 4. Start service
sudo systemctl enable gosync
sudo systemctl start gosync

# 5. Provision from existing S3 data
gosync sync provision / --recursive

# 6. Monitor
gosync sync state
```

### Development Workflow

```bash
# Run agent in foreground with debug logging
gosync agent --config ./dev-config.yaml

# In another terminal, test commands
gosync sync state
gosync sync provision /test --dry-run
gosync sync mirror ./test-data /test
gosync sync list --format json
```

---

## Summary

Your agent-based architecture is **excellent** for self-hosted scenarios. Key advantages:

### ✅ Strengths

1. **No Server Required**: Everything can run locally with SQLite
2. **Flexible Scaling**: Add PostgreSQL + Redis when needed
3. **Familiar Pattern**: Like HashiCorp tools (Consul, Nomad)
4. **Simple Coordination**: Database IS the coordination layer
5. **Easy Management**: CLI commands control running agent

### 🎯 Key Design Decisions

1. **Metadata Store Options**:
   - **SQLite**: Simple, single-user, development
   - **PostgreSQL**: Shared, multi-client, production

2. **Redis as Optional Cache**:
   - Not required, just performance boost
   - Cache metadata queries
   - Cache small file content

3. **Agent Architecture**:
   - Single binary, multiple modes
   - Unix socket for CLI ↔ Agent communication
   - Like Docker CLI ↔ Docker daemon

4. **Provision Command**:
   - Critical for existing S3 data
   - Populates metadata without downloading
   - Enables "adopt existing buckets"

### 🚀 Next Steps

1. **Start with Phase 1**: Agent infrastructure + RPC
2. **Add SQLite first**: Simpler for development
3. **Build provision command**: Essential for testing
4. **Then add PostgreSQL**: When multi-client needed

This architecture is much cleaner than a traditional centralized service and perfect for self-hosted/small team scenarios!