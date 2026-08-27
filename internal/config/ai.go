package config

// AIConfig holds configuration for the AI modules
type AIConfig struct {
	EncryptionKey   string // 32-byte hex-encoded key for provider key encryption
	UsageBufferSize int    // Async usage log buffer size
}

func loadAIConfig() AIConfig {
	return AIConfig{
		EncryptionKey:   getEnv("AI_ENCRYPTION_KEY", ""),
		UsageBufferSize: getEnvInt("AI_USAGE_BUFFER_SIZE", 1000),
	}
}
