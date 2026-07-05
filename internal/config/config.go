package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration so YAML strings like "10s" or "5m" unmarshal
// directly via time.ParseDuration instead of yaml.v3's default integer-nanosecond decoding.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

type Config struct {
	Server     Server     `yaml:"server"`
	Storage    Storage    `yaml:"storage"`
	Extraction Extraction `yaml:"extraction"`
	Ingestion  Ingestion  `yaml:"ingestion"`
	Rules      Rules      `yaml:"rules"`
	SLA        SLA        `yaml:"sla"`
	Outbox     Outbox     `yaml:"outbox"`
	DLQ        DLQ        `yaml:"dlq"`
	Logging    Logging    `yaml:"logging"`
}

type Server struct {
	Listen string `yaml:"listen"`
}

type Storage struct {
	SQLitePath string `yaml:"sqlite_path"`
}

type Extraction struct {
	Backend  string           `yaml:"backend"`
	API      APIExtraction    `yaml:"api"`
	Ollama   OllamaExtraction `yaml:"ollama"`
	Fallback Fallback         `yaml:"fallback"`
}

type APIExtraction struct {
	Provider       string         `yaml:"provider"`
	Model          string         `yaml:"model"`
	APIKeyEnv      string         `yaml:"api_key_env"`
	Timeout        Duration       `yaml:"timeout"`
	MaxRetries     int            `yaml:"max_retries"`
	CircuitBreaker CircuitBreaker `yaml:"circuit_breaker"`
}

type CircuitBreaker struct {
	FailureThreshold int      `yaml:"failure_threshold"`
	Cooldown         Duration `yaml:"cooldown"`
}

type OllamaExtraction struct {
	BaseURL string   `yaml:"base_url"`
	Model   string   `yaml:"model"`
	Timeout Duration `yaml:"timeout"`
}

type Fallback struct {
	MaxAttempts    int    `yaml:"max_attempts"`
	DefaultOutcome string `yaml:"default_outcome"`
}

type Ingestion struct {
	AppStore AppStoreIngestion `yaml:"app_store"`
	Email    EmailIngestion    `yaml:"email"`
}

type AppStoreIngestion struct {
	Enabled        bool     `yaml:"enabled"`
	IssuerID       string   `yaml:"issuer_id"`
	KeyID          string   `yaml:"key_id"`
	PrivateKeyPath string   `yaml:"private_key_path"`
	AppID          string   `yaml:"app_id"`
	PollInterval   Duration `yaml:"poll_interval"`
}

type EmailIngestion struct {
	Enabled      bool     `yaml:"enabled"`
	Mode         string   `yaml:"mode"`
	IMAP         IMAP     `yaml:"imap"`
	PollInterval Duration `yaml:"poll_interval"`
}

type IMAP struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password_env"`
	Mailbox     string `yaml:"mailbox"`
}

type Rules struct {
	Path         string `yaml:"path"`
	CELCostLimit int    `yaml:"cel_cost_limit"`
}

type SLA struct {
	P1 Duration `yaml:"P1"`
	P2 Duration `yaml:"P2"`
	P3 Duration `yaml:"P3"`
	P4 Duration `yaml:"P4"`
}

type Outbox struct {
	PollInterval Duration `yaml:"poll_interval"`
	MaxRetries   int      `yaml:"max_retries"`
}

type DLQ struct {
	RetentionDays int `yaml:"retention_days"`
}

type Logging struct {
	Level        string `yaml:"level"`
	AuditLogPath string `yaml:"audit_log_path"`
}

// Load reads and parses the config file at path into a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return &cfg, nil
}
