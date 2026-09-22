package smokescreen

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatsServerResolvesNilLogger(t *testing.T) {
	cfg := NewConfig()
	cfg.Log = nil
	server := newServer(cfg)
	require.NotNil(t, server.config.Log)
	require.IsType(t, &slog.JSONHandler{}, server.config.Log.Handler())
}
