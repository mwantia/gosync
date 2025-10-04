package server

type LogServerConfig struct {
	Level      string                  `mapstructure:"level"       yaml:"level"`
	TimeFormat string                  `mapstructure:"time_format" yaml:"time_format"`
	File       string                  `mapstructure:"file"        yaml:"file"`
	NoColor    bool                    `mapstructure:"no_color"    yaml:"no_color"`
	JSON       bool                    `mapstructure:"json"        yaml:"json"`
	NoTerminal bool                    `mapstructure:"no_terminal" yaml:"no_terminal"`
	Rotation   LogServerRotationConfig `mapstructure:"rotation"    yaml:"rotation"`
}

type LogServerRotationConfig struct {
	MaxSize    int  `mapstructure:"max_size"     yaml:"max_size"`
	MaxBackups int  `mapstructure:"max_backups"  yaml:"max_backups"`
	MaxAge     int  `mapstructure:"max_age"      yaml:"max_age"`
	Compress   bool `mapstructure:"compress"     yaml:"compress"`
}
