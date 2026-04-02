package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

const (
	AppName = "meta-cli"

	KeyAccessToken    = "access_token"
	KeyAppID          = "app_id"
	KeyDefaultAccount = "default_account"
	KeyAPIVersion     = "api_version"
	KeyOutputFormat   = "output_format"
)

var validOutputFormats = map[string]bool{
	"json":  true,
	"table": true,
	"csv":   true,
}

type Config struct {
	AccessToken    string
	AppID          string
	DefaultAccount string
	APIVersion     string
	OutputFormat   string
}

type ConfigStore struct {
	mu         sync.RWMutex
	v          *viper.Viper
	configPath string
	loaded     bool
	cached     *Config
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", AppName, "config.yaml")
	}
	return filepath.Join(home, ".config", AppName, "config.yaml")
}

func NewStore(configPath string) *ConfigStore {
	if configPath == "" {
		configPath = DefaultConfigPath()
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.SetDefault(KeyAPIVersion, "v21.0")
	v.SetDefault(KeyOutputFormat, "json")

	v.SetEnvPrefix("META")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv(KeyDefaultAccount, "META_AD_ACCOUNT")

	return &ConfigStore{
		v:          v,
		configPath: configPath,
	}
}

func (s *ConfigStore) ConfigFilePath() string {
	return s.configPath
}

func (s *ConfigStore) Load() (*Config, error) {
	s.mu.RLock()
	if s.loaded {
		cached := s.cached
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return s.cached, nil
	}

	if err := s.v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file %s: %w", s.configPath, err)
		}
	}

	cfg := &Config{
		AccessToken:    s.v.GetString(KeyAccessToken),
		AppID:          s.v.GetString(KeyAppID),
		DefaultAccount: s.v.GetString(KeyDefaultAccount),
		APIVersion:     s.v.GetString(KeyAPIVersion),
		OutputFormat:   s.v.GetString(KeyOutputFormat),
	}

	s.cached = cfg
	s.loaded = true
	return cfg, nil
}

func (s *ConfigStore) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.v.GetString(key)
}

func (s *ConfigStore) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.v.Set(key, value)
	if err := s.persist(); err != nil {
		return fmt.Errorf("persisting config: %w", err)
	}

	s.cached = nil
	s.loaded = false

	return nil
}

func (s *ConfigStore) Validate(c *Config) error {
	if c.AccessToken == "" {
		return errors.New("access_token is required: set META_ACCESS_TOKEN or run 'meta config set access_token <token>'")
	}
	if c.APIVersion == "" {
		return errors.New("api_version is required")
	}
	if c.OutputFormat != "" && !validOutputFormats[c.OutputFormat] {
		return fmt.Errorf("invalid output_format %q: must be one of json, table, csv", c.OutputFormat)
	}
	return nil
}

func (s *ConfigStore) persist() error {
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, ".config.tmp.*.yaml")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	if err := f.Close(); err != nil {
		return err
	}

	if err := s.v.WriteConfigAs(tmpName); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, s.configPath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return nil
}
