package server

import "github.com/spf13/viper"

func GetServerDefault() BaseServerConfig {
	return BaseServerConfig{
		ShutdownTimeout: "10s",

		Log: LogServerConfig{
			Level:      "INFO",
			TimeFormat: "2006-01-02 15:04:05",
			File:       "",
			NoColor:    false,
			JSON:       false,
			NoTerminal: false,
			Rotation: LogServerRotationConfig{
				MaxSize:    128,
				MaxBackups: 5,
				MaxAge:     16,
				Compress:   false,
			},
		},
	}
}

func setDefaults() {
	defaults := GetServerDefault()

	viper.SetDefault("shutdown_timeout", defaults.ShutdownTimeout)

	viper.SetDefault("log.level", defaults.Log.Level)
	viper.SetDefault("log.time_format", defaults.Log.TimeFormat)
	viper.SetDefault("log.file", defaults.Log.File)
	viper.SetDefault("log.no_color", defaults.Log.NoColor)
	viper.SetDefault("log.json", defaults.Log.JSON)
	viper.SetDefault("log.no_terminal", defaults.Log.NoTerminal)
	viper.SetDefault("log.rotation.max_size", defaults.Log.Rotation.MaxSize)
	viper.SetDefault("log.rotation.max_backups", defaults.Log.Rotation.MaxBackups)
	viper.SetDefault("log.rotation.max_age", defaults.Log.Rotation.MaxAge)
	viper.SetDefault("log.rotation.compress", defaults.Log.Rotation.Compress)
}
