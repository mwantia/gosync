# GoSync: Complete Vision - Virtual Filesystem + Dynamic Filters

## Table of Contents
1. [The Complete Vision](#the-complete-vision)
2. [Virtual Filesystem Architecture](#virtual-filesystem-architecture)
3. [Tag-Based Organization](#tag-based-organization)
4. [Dynamic Filters](#dynamic-filters)
5. [Integration & Use Cases](#integration--use-cases)
6. [Metadata Model](#metadata-model)
7. [Command Reference](#command-reference)

---

## The Complete Vision

### Three-Layer Architecture

```
┌─────────────────────────────────────────────────────────┐
│                 Virtual Filesystem                      │
│                                                         │
│  /                                                      │
│  ├── selfhosted/        ◄── Physical Backends (Example) │
│  │   ├── pictures/                                      │
│  │   └── documents/                                     │
│  ├── aws/                                               │
│  │   └── backups/                                       │
│  └── filters/           ◄── Dynamic Query (Example)     │
│      ├── pictures/                                      │
│      │   ├── red/       [tag:colour=red]                │
│      │   ├── vacation/  [tag:event=vacation]            │
│      │   └── 2024/      [year=2024]                     │
│      ├── videos/                                        │
│      │   └── 4k/        [tag:quality=4k]                │
│      └── documents/                                     │
│          ├── work/      [tag:category=work]             │
│          └── important/ [tag:priority=high]             │
│                                                         │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              Metadata + Tag Database                    │
│                                                         │
│  Files:                                                 │
│    - id, backend_id, path, size, ...                    │
│                                                         │
│  Tags:                                                  │
│    - file_id, key, value                                │
│    - Examples:                                          │
│      * colour=red, event=vacation                       │
│      * category=work, priority=high                     │
│                                                         │
│  Filters:                                               │
│    - id, virtual_path, query_expression                 │
│    - Automatically evaluate and update                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│              Physical S3 Backends                       │
│                                                         │
│  Files stored as-is, tags in metadata only              │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Core Concepts

**1. Physical Backends**
- Real S3 storage locations
- Files exist physically
- Traditional hierarchical structure

**2. Dynamic Filters**
- Virtual paths that represent queries
- No physical storage
- Real-time evaluation of metadata queries
- Automatically update as files are tagged/untagged

**3. Tags**
- Key-value metadata attached to files
- Stored in metadata DB, not filesystem
- Work across all backends
- Enable powerful filtering and organization

---

## Virtual Filesystem Architecture

### Path Types

Your virtual filesystem supports **three types of paths**:

```
Type 1: Backend Paths
  └─ selfhosted/pictures/vacation.jpg
     │         │         └─ File path
     │         └─ Prefix within backend
     └─ Backend identifier

Type 2: Filter Paths  
  └─ filters/pictures/red
     │       │        └─ Filter name
     │       └─ Category/organization
     └─ Special "filters" namespace

Type 3: Mixed Operations
  └─ Can sync between any path types:
     - Backend to Backend
     - Backend to Filter
     - Filter to Local
```

### How Filters Work

**Static Path**:
- Direct reference to a file
- Stored physically on backend
- Traditional file access

**Dynamic Filter**:
- Query definition: `tag:colour=red AND path LIKE '%/pictures/%'`
- Evaluates in real-time
- Returns all matching files from ANY backend
- Automatically updates as tags change

### Example Flow

```bash
# User lists a filter
gosync ls filters/pictures/red

# System:
1. Performs a lookup to vfs "filters/pictures/red" and identifies it as filter
2. Looks up filter definition
3. Executes query: SELECT * FROM files WHERE tags LIKE '%colour=red%'
4. Returns results from all backends
5. Display to user

# Result might include:
selfhosted/pictures/sunset.jpg    (tagged colour=red)
aws/photos/car.jpg                (tagged colour=red)
backblaze/images/flower.jpg       (tagged colour=red)
```

---

## Tag-Based Organization

### The Problem with Traditional Filesystems

**Hierarchical folders force single-dimension organization:**
```
/pictures/
  /vacation/
    /2024/
      /beach/
        sunset.jpg  ◄── Where does this go?
                        Vacation? 2024? Beach? Red sunset?
```

**You can only put it in ONE place**, but it might belong to multiple categories.

### Tag-Based Solution

**Same file with multiple dimensions:**
```
sunset.jpg
  Tags:
    - event: vacation
    - year: 2024
    - location: beach
    - colour: red, orange, yellow
    - time: evening
    - quality: 4k
```

Now accessible through **any filter**:
- `filters/pictures/vacation` - Shows this file
- `filters/pictures/red` - Shows this file
- `filters/pictures/2024` - Shows this file
- `filters/pictures/beach` - Shows this file

### Tag Management Commands

```bash
# Add tags to a file
gosync tag add selfhosted/pictures/sunset.jpg \
  colour=red colour=orange \
  event=vacation \
  location=beach

# List file tags
gosync tag list selfhosted/pictures/sunset.jpg

# Remove tags
gosync tag remove selfhosted/pictures/sunset.jpg colour=orange

# Bulk tagging (match pattern)
gosync tag add selfhosted/pictures/*.jpg year=2024

# Tag based on EXIF data (for images)
gosync tag auto selfhosted/pictures/ --exif

# Search by tags
gosync tag search colour=red event=vacation
```

---

## Dynamic Filters

### Filter Creation

```bash
# Create a simple filter
gosync filter create filters/pictures/red \
  --filter "tag:colour=red"

# Create complex filter
gosync filter create filters/work/urgent \
  --filter "tag:category=work AND tag:priority=high"

# Create filter with path constraints
gosync filter create filters/videos/4k \
  --filter "tag:quality=4k AND mime_type:video/* AND size>1GB"

# Create date-based filter
gosync filter create filters/recent/week \
  --filter "modified_time > now-7d"

# Create combined filter
gosync filter create filters/photos/best \
  --filter "tag:rating>=4 AND tag:favourite=true"
```

### Filter Query Language

**Simple tag matching:**
```
tag:colour=red              # Single tag
tag:event=vacation          # Another tag
tag:rating>=4               # Numeric comparison
```

**Logical operators:**
```
tag:colour=red AND tag:event=vacation
tag:priority=high OR tag:priority=urgent
tag:category=work AND NOT tag:archived=true
```

**Field-based filtering:**
```
mime_type:image/*           # All images
size>10MB                   # Files larger than 10MB
modified_time>2024-01-01    # Modified after date
path:/pictures/*            # Path matching
backend:selfhosted          # Specific backend
```

**Combined queries:**
```
tag:event=vacation AND mime_type:image/* AND size<5MB
(tag:colour=red OR tag:colour=blue) AND backend:selfhosted
tag:category=work AND modified_time>now-30d AND NOT tag:archived=true
```

### Filter Types

**1. Tag Filters** - Most common
```
filters/pictures/red         → tag:colour=red
filters/videos/4k            → tag:quality=4k
filters/documents/work       → tag:category=work
```

**2. Time-based Filters** - Dynamic
```
filters/recent/today         → modified_time>now-1d
filters/recent/week          → modified_time>now-7d
filters/archive/old          → modified_time<now-1y
```

**3. Content Filters** - Media type
```
filters/media/images         → mime_type:image/*
filters/media/videos         → mime_type:video/*
filters/media/large          → size>100MB
```

**4. Smart Filters** - Complex logic
```
filters/photos/best          → tag:rating>=4 AND tag:favourite=true
filters/work/pending         → tag:category=work AND tag:status=pending
filters/backup/priority      → tag:important=true AND NOT backed_up=true
```

---

## Integration & Use Cases

### Use Case 1: Photo Management

**Problem**: You have 50,000 photos across 3 storage backends. You want to organize by event, location, people, colour, etc., without duplicating files.

**Solution with Filters:**

```bash
# Create filter structure
gosync filter create filters/photos/vacation \
  --filter "tag:event=vacation"

gosync filter create filters/photos/family \
  --filter "tag:people contains 'family'"

gosync filter create filters/photos/red \
  --filter "tag:colour=red"

gosync filter create filters/photos/favourites \
  --filter "tag:favourite=true AND tag:rating>=4"

# Tag photos (can be automated with AI/EXIF)
gosync tag add selfhosted/photos/2024/*.jpg \
  event=vacation location=beach year=2024

# Browse by any dimension
gosync ls filters/photos/vacation     # All vacation photos
gosync ls filters/photos/beach        # All beach photos
gosync ls filters/photos/red          # All photos with red

# Mirror favourites locally
gosync mirror ~/Desktop/Favourites filters/photos/favourites
# Automatically syncs any photo tagged as favourite!
```

**Result**: Same photos accessible through multiple organizational lenses without duplication.

### Use Case 2: Work Document Organization

**Problem**: Documents belong to multiple projects, categories, and priorities simultaneously.

**Solution:**

```bash
# Create work filters
gosync filter create filters/work/contracts \
  --filter "tag:type=contract AND tag:status=active"

gosync filter create filters/work/urgent \
  --filter "tag:priority=high AND tag:status=pending"

gosync filter create filters/work/project-alpha \
  --filter "tag:project=alpha"

# Tag documents
gosync tag add aws/docs/agreement.pdf \
  type=contract status=active priority=high project=alpha

# This document now appears in:
# - filters/work/contracts
# - filters/work/urgent  
# - filters/work/project-alpha

# Mirror urgent work items locally
gosync mirror ~/Desktop/Urgent filters/work/urgent
```

### Use Case 3: Media Library Management

**Problem**: Building a media server with content organized by genre, quality, year, language.

**Solution:**

```bash
# Create media filters
gosync filter create filters/movies/4k \
  --filter "tag:quality=4k AND mime_type:video/*"

gosync filter create filters/movies/scifi \
  --filter "tag:genre=scifi"

gosync filter create filters/movies/recent \
  --filter "tag:year>=2020"

gosync filter create filters/movies/unwatched \
  --filter "tag:watched=false"

# Tag movies
gosync tag add selfhosted/media/movies/*.mkv \
  quality=4k genre=scifi year=2024

# Plex/Jellyfin can monitor local mirrors
gosync mirror /mnt/media/4k filters/movies/4k
gosync mirror /mnt/media/scifi filters/movies/scifi
```

### Use Case 4: Development Project Management

**Problem**: Code, docs, builds spread across repos, but need logical grouping.

**Solution:**

```bash
# Create dev filters
gosync filter create filters/projects/active \
  --filter "tag:status=active AND tag:type=project"

gosync filter create filters/releases/staging \
  --filter "tag:environment=staging AND tag:type=build"

gosync filter create filters/docs/api \
  --filter "tag:category=documentation AND tag:type=api"

# Tag files across backends
gosync tag add selfhosted/repos/api/*.go \
  type=project status=active language=go

gosync tag add aws/builds/v2.1.0/* \
  type=build environment=staging version=2.1.0

# Mirror active projects to local dev
gosync mirror ~/Work/Active filters/projects/active
```

### Use Case 5: Backup Priority System

**Problem**: Not all files need frequent backups. Want tiered backup strategy.

**Solution:**

```bash
# Create priority filters
gosync filter create filters/backup/critical \
  --filter "tag:backup=critical"

gosync filter create filters/backup/important \
  --filter "tag:backup=important"

gosync filter create filters/backup/normal \
  --filter "tag:backup=normal"

# Tag files by importance
gosync tag add selfhosted/documents/financial/* \
  backup=critical

gosync tag add selfhosted/photos/* \
  backup=important

# Set up cascading backups
gosync mirror-remote filters/backup/critical aws/backup-hourly
gosync mirror-remote filters/backup/important aws/backup-daily
gosync mirror-remote filters/backup/normal backblaze/backup-weekly
```

---

## Metadata Model

### Core Entities

**Files Table**
```
id, backend_id, path, size, mime_type, 
modified_time, checksum, etag, ...
```

**Tags Table**
```
file_id, key, value

Examples:
  - (123, "colour", "red")
  - (123, "event", "vacation")
  - (456, "category", "work")
  - (456, "priority", "high")
```

**Filters Table**
```
id, virtual_path, query_expression, created_at, updated_at

Examples:
  - ("filters/pictures/red", "tag:colour=red", ...)
  - ("filters/work/urgent", "tag:category=work AND tag:priority=high", ...)
```

**Filter Cache Table** (Optional optimization)
```
filter_id, file_id, last_updated

Pre-computed filter results for fast listing
Invalidated when tags change
```

### Tag Indexing

**For performance, tags need proper indexing:**

PostgreSQL:
```sql
-- GIN index for fast tag lookups
CREATE INDEX idx_tags_gin ON tags USING gin ((key || '=' || value));

-- Or JSONB if storing tags as JSON
ALTER TABLE files ADD COLUMN tags JSONB;
CREATE INDEX idx_files_tags ON files USING gin (tags);
```

SQLite:
```sql
-- FTS for tag search
CREATE VIRTUAL TABLE tags_fts USING fts5(
  key, value, file_id
);

-- Regular index
CREATE INDEX idx_tags_key_value ON tags(key, value);
```

### Filter Evaluation

When a filter path is accessed:

1. **Parse Query**: Convert filter expression to SQL
2. **Execute**: Run query against metadata DB
3. **Return Results**: List of matching files
4. **Cache** (optional): Store results for repeated access

Example transformation:
```
Filter: tag:colour=red AND tag:event=vacation
   ↓
SQL: SELECT f.* FROM files f
     JOIN tags t1 ON f.id = t1.file_id AND t1.key='colour' AND t1.value='red'
     JOIN tags t2 ON f.id = t2.file_id AND t2.key='event' AND t2.value='vacation'
```

---

## Command Reference

### Tag Commands

```bash
# Add tags
gosync tag add <path> <key>=<value> [<key>=<value>...]
gosync tag add selfhosted/pic.jpg colour=red event=vacation

# List tags
gosync tag list <path>
gosync tag list selfhosted/pic.jpg

# Remove tags
gosync tag remove <path> <key> [<key>...]
gosync tag remove selfhosted/pic.jpg colour

# Update tag value
gosync tag set <path> <key>=<new-value>
gosync tag set selfhosted/pic.jpg colour=blue

# Search by tags
gosync tag search <key>=<value> [AND/OR <key>=<value>...]
gosync tag search colour=red event=vacation

# Bulk operations
gosync tag add selfhosted/photos/*.jpg year=2024
gosync tag auto selfhosted/photos/ --exif  # Auto-tag from EXIF
```

### Filter Commands

```bash
# Create filter
gosync filter create <virtual-path> --filter "<query>"
gosync filter create filters/photos/red --filter "tag:colour=red"

# List all filters
gosync filter list
gosync filter list --json

# Show filter definition
gosync filter show filters/photos/red

# Update filter
gosync filter update filters/photos/red --filter "tag:colour=red OR tag:colour=crimson"

# Delete filter
gosync filter delete filters/photos/red

# Test filter (see what it would match)
gosync filter test "tag:colour=red AND mime_type:image/*"
```

### Mirror with Filters

```bash
# Mirror filter to local directory
gosync mirror ~/Local/Path filters/photos/red

# Mirror between filter and backend
gosync mirror-remote filters/photos/best aws/backups/best-photos

# Bidirectional mirroring not available for filters
# (filters are read-only views)
```

### Combined Workflows

```bash
# Workflow: Organize photos by AI tags
1. gosync scan selfhosted --prefix photos/
2. gosync tag auto selfhosted/photos/ --ai-labels
3. gosync filter create filters/photos/cats --filter "tag:ai-label=cat"
4. gosync ls filters/photos/cats
5. gosync mirror ~/Desktop/Cats filters/photos/cats

# Workflow: Work priority system
1. gosync tag add aws/docs/*.pdf category=work
2. gosync tag add aws/docs/contract*.pdf priority=high
3. gosync filter create filters/work/urgent --filter "tag:priority=high"
4. gosync mirror ~/Desktop/Urgent filters/work/urgent

# Workflow: Backup automation
1. Tag files by importance
2. Create filters by backup tier
3. Mirror filters to different backup backends
4. Automated with different frequencies
```

---

## Advanced Features

### Auto-Tagging

**From EXIF (Photos)**
```bash
gosync tag auto selfhosted/photos/ --exif
# Extracts: date, camera, location, etc.
```

**From AI/ML**
```bash
gosync tag auto selfhosted/photos/ --ai-labels
# Adds: object detection, scene classification, colours
```

**From File Metadata**
```bash
gosync tag auto selfhosted/documents/ --file-meta
# Extracts: author, title, creation date from PDFs/Office docs
```

**From Filesystem**
```bash
gosync tag auto selfhosted/ --path-based
# Creates tags from directory structure
# /photos/2024/vacation/ → year=2024, event=vacation
```

### Filter Monitoring

Filters automatically stay current:

```bash
# Create filter
gosync filter create filters/recent/new --filter "modified_time>now-1h"

# Mirror it
gosync mirror ~/Desktop/Recent filters/recent/new

# As new files are added/modified matching the filter,
# they automatically appear in ~/Desktop/Recent
# No manual intervention needed!
```

### Smart Combinations

**Combine physical and virtual paths:**

```bash
# Mirror specific backend path to filter-based local structure
gosync mirror ~/Organized/Red selfhosted/photos/
  --filter "tag:colour=red"

# Mirror all vacation photos from all backends to one local folder
gosync mirror ~/Photos/Vacation filters/vacation/all

# Create multi-backend filter
gosync filter create filters/all-videos/4k \
  --filter "mime_type:video/* AND tag:quality=4k"
# This returns 4k videos from ALL backends
```

---

## Why This Architecture is Powerful

### 1. **Multi-Dimensional Organization**
Traditional: One file = One location
GoSync: One file = Multiple views

### 2. **No Duplication**
Files exist once physically, accessible through unlimited filters

### 3. **Dynamic & Automatic**
Filters update automatically as tags change

### 4. **Cross-Backend**
Filters work across all storage backends simultaneously

### 5. **Flexible Mirroring**
Sync filtered views locally, perfect for workflows

### 6. **Future-Proof**
Add new organizational dimensions anytime by creating new filters

### 7. **Works with Existing Storage**
Files remain as-is on S3, tags only in metadata

---

## Summary

Your complete vision combines:

**1. Virtual Filesystem** - Unified namespace for multiple S3 backends
**2. Dynamic Filters** - Query-based virtual paths
**3. Tag System** - Multi-dimensional file organization
**4. Flexible Mirroring** - Sync any path type to any location

This creates a system where:
- Files are stored once
- Organized by unlimited dimensions (tags)
- Accessible through multiple views (filters)
- Automatically synchronized (mirrors)
- Works across all storage (backends)

**It's like having smart playlists/folders for your entire storage infrastructure!**