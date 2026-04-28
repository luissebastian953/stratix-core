package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              string
	DBUrl             string
	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration
	SchedulerInterval time.Duration

	JWTSecret           string
	JWTAccessExpMinutes int
	JWTRefreshExpDays   int

	AnthropicAPIKey     string
	AnthropicAPIURL     string
	AnthropicModel      string
	AnthropicAPIVersion string
	AnthropicMaxTokens  int
	AnthropicTimeoutSec int

	AIRateLimitRequests int
	AIRateLimitWindow   time.Duration

	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
}

func LoadConfig() Config {
	return Config{
		Port:              os.Getenv("APP_PORT"),
		DBUrl:             os.Getenv("DATABASE_URL"),
		DBMaxConns:        int32(envInt("DB_MAX_CONNS", 10)),
		DBMinConns:        int32(envInt("DB_MIN_CONNS", 2)),
		DBMaxConnLifetime: envDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		DBMaxConnIdleTime: envDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
		SchedulerInterval: 1 * time.Minute,

		JWTSecret:           os.Getenv("JWT_SECRET"),
		JWTAccessExpMinutes: envInt("JWT_ACCESS_EXP_MINUTES", 15),
		JWTRefreshExpDays:   envInt("JWT_REFRESH_EXP_DAYS", 30),

		AnthropicAPIKey:     os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicAPIURL:     envStr("ANTHROPIC_API_URL", "https://api.anthropic.com/v1/messages"),
		AnthropicModel:      envStr("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001"),
		AnthropicAPIVersion: envStr("ANTHROPIC_API_VERSION", "2023-06-01"),
		AnthropicMaxTokens:  envInt("ANTHROPIC_MAX_TOKENS", 512),
		AnthropicTimeoutSec: envInt("ANTHROPIC_TIMEOUT_SECONDS", 15),

		AIRateLimitRequests: envInt("AI_RATE_LIMIT_REQUESTS", 20),
		AIRateLimitWindow:   envDuration("AI_RATE_LIMIT_WINDOW", time.Hour),

		R2AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:          os.Getenv("R2_BUCKET"),
	}
}

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
