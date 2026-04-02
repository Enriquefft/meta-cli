package config

import (
	"os"
	"path/filepath"
	"testing"
)

func unsetMetaEnvs(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"META_ACCESS_TOKEN",
		"META_APP_ID",
		"META_AD_ACCOUNT",
		"META_API_VERSION",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestLoadDefaults(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	SetConfigPath(filepath.Join(tmp, "config.yaml"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIVersion != "v21.0" {
		t.Errorf("expected default api_version v21.0, got %s", cfg.APIVersion)
	}
	if cfg.OutputFormat != "json" {
		t.Errorf("expected default output_format json, got %s", cfg.OutputFormat)
	}
	if cfg.AccessToken != "" {
		t.Errorf("expected empty access_token with no config, got %s", cfg.AccessToken)
	}
}

func TestLoadFromEnv(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	SetConfigPath(filepath.Join(tmp, "config.yaml"))

	t.Setenv("META_ACCESS_TOKEN", "test-token")
	t.Setenv("META_APP_ID", "test-app-id")
	t.Setenv("META_API_VERSION", "v20.0")
	t.Setenv("META_AD_ACCOUNT", "act_999")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AccessToken != "test-token" {
		t.Errorf("expected access_token from env, got %s", cfg.AccessToken)
	}
	if cfg.AppID != "test-app-id" {
		t.Errorf("expected app_id from env, got %s", cfg.AppID)
	}
	if cfg.APIVersion != "v20.0" {
		t.Errorf("expected api_version from env, got %s", cfg.APIVersion)
	}
	if cfg.DefaultAccount != "act_999" {
		t.Errorf("expected default_account from META_AD_ACCOUNT env, got %s", cfg.DefaultAccount)
	}
}

func TestLoadFromFile(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")

	content := "access_token: file-token\ndefault_account: act_456\noutput_format: table\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	SetConfigPath(cfgFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AccessToken != "file-token" {
		t.Errorf("expected access_token from file, got %s", cfg.AccessToken)
	}
	if cfg.DefaultAccount != "act_456" {
		t.Errorf("expected default_account from file, got %s", cfg.DefaultAccount)
	}
	if cfg.OutputFormat != "table" {
		t.Errorf("expected output_format from file, got %s", cfg.OutputFormat)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")

	content := "access_token: file-token\napi_version: v19.0\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	SetConfigPath(cfgFile)

	t.Setenv("META_ACCESS_TOKEN", "env-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AccessToken != "env-token" {
		t.Errorf("env should override file, got %s", cfg.AccessToken)
	}
	if cfg.APIVersion != "v19.0" {
		t.Errorf("file value should be used when no env, got %s", cfg.APIVersion)
	}
}

func TestSetAndGet(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	SetConfigPath(filepath.Join(tmp, "config.yaml"))
	Load()

	err := Set("default_account", "act_789")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val := Get("default_account")
	if val != "act_789" {
		t.Errorf("Get returned %s, want act_789", val)
	}

	Reset()
	unsetMetaEnvs(t)
	SetConfigPath(filepath.Join(tmp, "config.yaml"))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DefaultAccount != "act_789" {
		t.Errorf("expected persisted default_account act_789, got %s", cfg.DefaultAccount)
	}
}

func TestSetAndGet_METAAdAccount(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	SetConfigPath(filepath.Join(tmp, "config.yaml"))
	Load()

	err := Set("default_account", "act_persisted")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val := Get("default_account")
	if val != "act_persisted" {
		t.Errorf("Get returned %s, want act_persisted", val)
	}

	Reset()
	t.Setenv("META_AD_ACCOUNT", "act_from_env")
	SetConfigPath(filepath.Join(tmp, "config.yaml"))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DefaultAccount != "act_from_env" {
		t.Errorf("META_AD_ACCOUNT env should override persisted value, got %s", cfg.DefaultAccount)
	}
}

func TestValidate_MissingToken(t *testing.T) {
	cfg := &Config{}
	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for missing access_token")
	}
}

func TestValidate_InvalidOutputFormat(t *testing.T) {
	cfg := &Config{
		AccessToken:  "valid-token",
		APIVersion:   "v21.0",
		OutputFormat: "xml",
	}
	err := Validate(cfg)
	if err == nil {
		t.Error("expected error for invalid output_format")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{AccessToken: "valid-token", APIVersion: "v21.0"}
	err := Validate(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetCreatesDirectory(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	SetConfigPath(filepath.Join(tmp, "sub", "dir", "config.yaml"))
	Load()

	err := Set("access_token", "nested-token")
	if err != nil {
		t.Fatalf("Set failed with nested dirs: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, "sub", "dir", "config.yaml")); os.IsNotExist(err) {
		t.Error("config file not created in nested directory")
	}
}

func TestMalformedYAML(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")

	content := "access_token: \"unclosed\napi_version: v21.0\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	SetConfigPath(cfgFile)

	_, err := Load()
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestEmptyConfigFile(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")

	if err := os.WriteFile(cfgFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	SetConfigPath(cfgFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.APIVersion != "v21.0" {
		t.Errorf("expected default api_version v21.0, got %s", cfg.APIVersion)
	}
	if cfg.OutputFormat != "json" {
		t.Errorf("expected default output_format json, got %s", cfg.OutputFormat)
	}
}

func TestMETAAdAccountOverridesFile(t *testing.T) {
	Reset()
	unsetMetaEnvs(t)
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "config.yaml")

	content := "default_account: act_from_file\n"
	if err := os.WriteFile(cfgFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	SetConfigPath(cfgFile)

	t.Setenv("META_AD_ACCOUNT", "act_from_env")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DefaultAccount != "act_from_env" {
		t.Errorf("env META_AD_ACCOUNT should override file, got %s", cfg.DefaultAccount)
	}
}
