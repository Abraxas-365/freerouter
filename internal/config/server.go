package config

type ServerConfig struct {
	Port        int
	Environment string
	LogLevel    string
	BaseURL     string
	CORSOrigins []string
}

func loadServerConfig() ServerConfig {
	return ServerConfig{
		Port:        getEnvInt("SERVER_PORT", 8080),
		Environment: getEnv("ENVIRONMENT", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
		CORSOrigins: getEnvStringSlice("CORS_ORIGINS", []string{"http://localhost:3000"}),
	}
}
