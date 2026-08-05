package config

import (
	"time"

	"fmt"
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	AppName       string
	HHToken       string
	SearchText    string
	SearchArea    string
	ApplyInterval time.Duration
}

func getRequiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "",
			fmt.Errorf("required environment variable %s is not set",
				key)
	}
	return value, nil
}

func NewConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	token, err := getRequiredEnv("HH_TOKEN")
	if err != nil {
		return nil, fmt.Errorf("failed to get HH_TOKEN: %w", err)
	}

	appName := os.Getenv("APP_NAME")

	if appName == "" {
		appName = "HH Auto Apply"
	}

	searchText, err := getRequiredEnv("SEARCH_TEXT")
	if err != nil {
		return nil, err
	}

	intervalStr, err := getRequiredEnv("APPLY_INTERVAL")
	if err != nil {
		return nil, err
	}

	interval, err := time.ParseDuration(intervalStr)
	if nil != err {
		return nil, fmt.Errorf("invalid APPLY_INTERVAL: %w", err)
	}

	return &Config{
		AppName:       appName,
		HHToken:       token,
		SearchText:    searchText,
		ApplyInterval: interval,
	}, nil
}
