package config

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	// EnvPrefix is the environment variable namespace for CLI overrides.
	EnvPrefix = "CHAINOPS"

	keyConfigFile     = "config"
	keySpecFile       = "file"
	keyStateDir       = "state-dir"
	keyArtifactsDir   = "artifacts-dir"
	keyOutput         = "output"
	keyNonInteractive = "non-interactive"
	keyYes            = "yes"
)

// Values contains resolved CLI configuration after precedence resolution.
type Values struct {
	ConfigFile     string
	SpecFile       string
	StateDir       string
	ArtifactsDir   string
	Output         string
	NonInteractive bool
	Yes            bool
}

// Config resolves defaults, config file, env vars, and flags.
type Config struct {
	v                *viper.Viper
	defaults         Values
	loadedConfigFile string
}

// DefaultValues returns defaults tuned for chainops or legacy bgorch mode.
func DefaultValues(legacy bool) Values {
	stateDir := ".chainops/state"
	artifactsDir := ".chainops/render"
	if legacy {
		stateDir = ".bgorch/state"
		artifactsDir = ".bgorch/render"
	}
	return Values{
		ConfigFile:     "",
		SpecFile:       "chainops.yaml",
		StateDir:       stateDir,
		ArtifactsDir:   artifactsDir,
		Output:         "table",
		NonInteractive: false,
		Yes:            false,
	}
}

// New creates a configuration resolver bound to environment defaults.
func New(legacy bool) *Config {
	defaults := DefaultValues(legacy)
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	v.SetDefault(keyConfigFile, defaults.ConfigFile)
	v.SetDefault(keySpecFile, defaults.SpecFile)
	v.SetDefault(keyStateDir, defaults.StateDir)
	v.SetDefault(keyArtifactsDir, defaults.ArtifactsDir)
	v.SetDefault(keyOutput, defaults.Output)
	v.SetDefault(keyNonInteractive, defaults.NonInteractive)
	v.SetDefault(keyYes, defaults.Yes)

	return &Config{v: v, defaults: defaults}
}

// BindRootFlags binds known CLI keys to root persistent flags.
func (c *Config) BindRootFlags(flags *pflag.FlagSet) {
	for _, key := range []string{
		keyConfigFile,
		keySpecFile,
		keyStateDir,
		keyArtifactsDir,
		keyNonInteractive,
		keyYes,
	} {
		if flag := flags.Lookup(key); flag != nil {
			_ = c.v.BindPFlag(key, flag)
		}
	}
}

// Resolve merges defaults, optional config file, env vars and flags.
func (c *Config) Resolve() (Values, string, error) {
	explicitPath := strings.TrimSpace(c.v.GetString(keyConfigFile))
	loadedPath, err := c.loadConfigFile(explicitPath)
	if err != nil {
		return Values{}, "", err
	}
	c.loadedConfigFile = loadedPath

	values := Values{
		ConfigFile:     strings.TrimSpace(c.v.GetString(keyConfigFile)),
		SpecFile:       strings.TrimSpace(c.v.GetString(keySpecFile)),
		StateDir:       strings.TrimSpace(c.v.GetString(keyStateDir)),
		ArtifactsDir:   strings.TrimSpace(c.v.GetString(keyArtifactsDir)),
		Output:         strings.TrimSpace(c.v.GetString(keyOutput)),
		NonInteractive: c.v.GetBool(keyNonInteractive),
		Yes:            c.v.GetBool(keyYes),
	}

	if values.SpecFile == "" {
		values.SpecFile = c.defaults.SpecFile
	}
	if values.StateDir == "" {
		values.StateDir = c.defaults.StateDir
	}
	if values.ArtifactsDir == "" {
		values.ArtifactsDir = c.defaults.ArtifactsDir
	}
	if values.Output == "" {
		values.Output = c.defaults.Output
	}

	return values, c.loadedConfigFile, nil
}

// OutputFromConfig returns configured default output format.
func (c *Config) OutputFromConfig() string {
	value := strings.TrimSpace(c.v.GetString(keyOutput))
	if value == "" {
		return c.defaults.Output
	}
	return value
}

func (c *Config) loadConfigFile(explicitPath string) (string, error) {
	if explicitPath == "" {
		return "", nil
	}
	c.v.SetConfigFile(explicitPath)
	if err := c.v.ReadInConfig(); err != nil {
		return "", fmt.Errorf("read config file %q: %w", explicitPath, err)
	}
	return c.v.ConfigFileUsed(), nil
}
