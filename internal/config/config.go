package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config encapsulates all config required by the application.
type Config struct {
	HttpServer struct {
		Addr           string   `json:"addr"`
		AllowedOrigins []string `json:"allowedOrigins"`
		// Read here: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Access-Control-Max-Age
		CorsMaxAgeSec int `json:"corsMaxAgeSec"`
	} `json:"httpServer"`

	Logger struct {
		// Leave empty for stdout logging.
		FilePath string `json:"filePath"`
		Level    string `json:"level"`
		Pretty   bool   `json:"pretty"`
	} `json:"logger"`
}

// Load config from the given JSON file.
func Load(jsonPath string) (Config, error) {
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file at %s because: %w", jsonPath, err)
	}

	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config file at %s because: %w", jsonPath, err)
	}

	if err := validate(config); err != nil {
		return Config{}, fmt.Errorf("config is invalid: %w", err)
	}

	return config, nil
}

// validate the loaded config.
func validate(conf Config) error {
	if conf.HttpServer.Addr == "" {
		return fmt.Errorf("httpServer.addr is required")
	}
	if len(conf.HttpServer.AllowedOrigins) == 0 {
		return fmt.Errorf("httpServer.allowedOrigins are required")
	}
	if conf.HttpServer.CorsMaxAgeSec < 1 {
		return fmt.Errorf("httpServer.corsMaxAgeSec is required")
	}

	if conf.Logger.Level == "" {
		return fmt.Errorf("logger.level is required")
	}

	return nil
}
