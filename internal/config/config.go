package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config encapsulates all config required by the application.
type Config struct {
	Database struct {
		Addr     string `json:"addr"`
		Username string `json:"username"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`

	HttpServer struct {
		Addr           string   `json:"addr"`
		AllowedOrigins []string `json:"allowedOrigins"`
		// Read here: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Access-Control-Max-Age
		CorsMaxAgeSec int `json:"corsMaxAgeSec"`
	} `json:"httpServer"`

	Kafka struct {
		Brokers          []string `json:"brokers"`
		Username         string   `json:"username"`
		Password         string   `json:"password"`
		CACertPath       string   `json:"caCertPath"`
		ValidationsTopic string   `json:"validationsTopic"`
		LedgerTopic      string   `json:"ledgerTopic"`
		ConsumerGroupID  string   `json:"consumerGroupID"`

		MaxMessageRetryCount   int `json:"maxMessageRetryCount"`
		MessageRetryIntervalMs int `json:"messageRetryIntervalMs"`
	} `json:"kafka"`

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
	if conf.Database.Addr == "" {
		return fmt.Errorf("database.addr is required")
	}
	if conf.Database.Username == "" {
		return fmt.Errorf("database.username is required")
	}
	if conf.Database.Password == "" {
		return fmt.Errorf("database.password is required")
	}
	if conf.Database.Database == "" {
		return fmt.Errorf("database.database is required")
	}

	if conf.HttpServer.Addr == "" {
		return fmt.Errorf("httpServer.addr is required")
	}
	if len(conf.HttpServer.AllowedOrigins) == 0 {
		return fmt.Errorf("httpServer.allowedOrigins are required")
	}
	if conf.HttpServer.CorsMaxAgeSec < 1 {
		return fmt.Errorf("httpServer.corsMaxAgeSec is required")
	}

	if len(conf.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers are required")
	}
	if conf.Kafka.ValidationsTopic == "" {
		return fmt.Errorf("kafka.validationsTopic is required")
	}
	if conf.Kafka.LedgerTopic == "" {
		return fmt.Errorf("kafka.ledgerTopic is required")
	}
	if conf.Kafka.ConsumerGroupID == "" {
		return fmt.Errorf("kafka.consumerGroupID is required")
	}
	if conf.Kafka.MaxMessageRetryCount < 1 {
		return fmt.Errorf("kafka.maxMessageRetryCount should be > 0")
	}
	if conf.Kafka.MessageRetryIntervalMs < 1 {
		return fmt.Errorf("kafka.messageRetryIntervalMs should be > 0")
	}

	if conf.Logger.Level == "" {
		return fmt.Errorf("logger.level is required")
	}

	return nil
}
