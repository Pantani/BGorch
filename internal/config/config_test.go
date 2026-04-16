package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestResolvePrecedenceDefaultsConfigEnvFlags(t *testing.T) {
	t.Setenv("CHAINOPS_STATE_DIR", "/tmp/from-env")
	t.Setenv("CHAINOPS_OUTPUT", "yaml")

	configPath := filepath.Join(t.TempDir(), "chainops-cli.yaml")
	configContent := "file: from-config.yaml\n" +
		"state-dir: /tmp/from-config\n" +
		"artifacts-dir: /tmp/artifacts-config\n" +
		"output: json\n" +
		"yes: true\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolver := New(false)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("config", "", "")
	flags.String("file", "", "")
	flags.String("state-dir", "", "")
	flags.String("artifacts-dir", "", "")
	flags.Bool("non-interactive", false, "")
	flags.Bool("yes", false, "")
	resolver.BindRootFlags(flags)

	if err := flags.Set("config", configPath); err != nil {
		t.Fatalf("set config flag: %v", err)
	}
	if err := flags.Set("state-dir", "/tmp/from-flag"); err != nil {
		t.Fatalf("set state-dir flag: %v", err)
	}
	if err := flags.Set("yes", "false"); err != nil {
		t.Fatalf("set yes flag: %v", err)
	}

	resolved, loadedPath, err := resolver.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if loadedPath != configPath {
		t.Fatalf("loaded config path mismatch: got %q want %q", loadedPath, configPath)
	}
	if resolved.SpecFile != "from-config.yaml" {
		t.Fatalf("spec file precedence mismatch: got %q", resolved.SpecFile)
	}
	if resolved.StateDir != "/tmp/from-flag" {
		t.Fatalf("state dir precedence mismatch: got %q", resolved.StateDir)
	}
	if resolved.ArtifactsDir != "/tmp/artifacts-config" {
		t.Fatalf("artifacts dir precedence mismatch: got %q", resolved.ArtifactsDir)
	}
	if resolved.Output != "yaml" {
		t.Fatalf("output precedence mismatch (env should win): got %q", resolved.Output)
	}
	if resolved.Yes {
		t.Fatalf("yes precedence mismatch (flag false should win over config true)")
	}
}
