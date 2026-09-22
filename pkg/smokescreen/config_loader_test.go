package smokescreen

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stripe/smokescreen/internal/testlog"
	"gopkg.in/yaml.v2"
)

func TestLoadConfigWithLogger(t *testing.T) {
	logger, records := testlog.New()
	logger = logger.With(slog.String("component", "caller"))
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("acl_file: testdata/acl.yaml\n"), 0600))
	cfg, err := LoadConfigWithLogger(path, logger)
	require.NoError(t, err)
	require.Same(t, logger, cfg.Log)
	require.NotNil(t, cfg.EgressACL)
	require.NotEmpty(t, records.AllEntries())
	for _, entry := range records.AllEntries() {
		require.Equal(t, "caller", entry.Data["component"])
	}
}

func TestLoadConfigDefaultLogger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0600))
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	require.IsType(t, &slog.JSONHandler{}, cfg.Log.Handler())
	require.Equal(t, uint16(DefaultPort), cfg.Port)
}

func TestYAMLStillResetsSettingsExceptLogger(t *testing.T) {
	logger, _ := testlog.New()
	cfg := NewConfig()
	cfg.Log = logger
	cfg.Port = 9999
	cfg.AllowMissingRole = true
	cfg.SupportProxyProtocol = true
	cfg.UnsafeAllowPrivateRanges = true
	require.NoError(t, cfg.SetAllowAddresses([]string{"192.0.2.1"}))
	require.NoError(t, cfg.SetRateLimits(3, 5, 10))
	require.NoError(t, yaml.UnmarshalStrict([]byte("max_request_burst: 5\nread_timeout: 0s\n"), cfg))
	require.Same(t, logger, cfg.Log)
	require.Equal(t, uint16(DefaultPort), cfg.Port)
	require.False(t, cfg.AllowMissingRole)
	require.False(t, cfg.SupportProxyProtocol)
	require.False(t, cfg.UnsafeAllowPrivateRanges)
	require.Empty(t, cfg.AllowRanges)
	require.Zero(t, cfg.MaxConcurrentRequests)
	require.Zero(t, cfg.MaxRequestRate)
	require.Zero(t, cfg.MaxRequestBurst, "burst alone remains ignored")
	require.Equal(t, DefaultReadTimeout, cfg.ReadTimeout, "zero retains the existing timeout default")
}

func TestInvalidSocketModeStillExits(t *testing.T) {
	const env = "SMOKESCREEN_TEST_INVALID_SOCKET_MODE"
	if path := os.Getenv(env); path != "" {
		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil)).With(slog.String("component", "caller"))
		_, _ = LoadConfigWithLogger(path, logger)
		return
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("stats_socket_file_mode: invalid\n"), 0600))
	cmd := exec.Command(os.Args[0], "-test.run=^TestInvalidSocketModeStillExits$")
	cmd.Env = append(os.Environ(), env+"="+path)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 1, exit.ExitCode())
	require.Contains(t, string(out), `"level":"ERROR"`)
	require.Contains(t, string(out), `"component":"caller"`)
	require.Contains(t, string(out), "invalid syntax")
}
