package config

import "os"

// EnvOrDefault returns the value of the environment variable if present, otherwise fallback.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
