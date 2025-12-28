package main

import (
	"os"
	"strconv"
)

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, parseErr := strconv.Atoi(value)
		if parseErr == nil {
			return intValue
		}
	}
	return defaultValue
}
