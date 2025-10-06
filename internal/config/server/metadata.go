package server

// MetadataServerConfig holds metadata store configuration
type MetadataServerConfig struct {
	Type   string               `mapstructure:"type"   yaml:"type"`
	SQLite MetadataSQLiteConfig `mapstructure:"sqlite" yaml:"sqlite"`
}

// SQLiteMetadataConfig holds SQLite-specific configuration
type MetadataSQLiteConfig struct {
	Path string `mapstructure:"path" yaml:"path"`
}
