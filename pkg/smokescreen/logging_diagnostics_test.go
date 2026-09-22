package smokescreen

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/goproxy"
	acl "github.com/stripe/smokescreen/pkg/smokescreen/acl/v1"
)

type failedResponseWriter struct{}

func (failedResponseWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func (failedResponseWriter) Close() error              { return errors.New("close failed") }

func TestResponseDiagnosticsKeepCurrentRequestFields(t *testing.T) {
	var output bytes.Buffer
	cfg := NewConfig()
	cfg.Log = slog.New(slog.NewJSONHandler(&output, nil))
	pctx := canonicalFixture(cfg)
	sctx := pctx.UserData.(*SmokescreenContext)
	pctx.ConnectAction = goproxy.ConnectMitm
	sctx.Decision.MitmConfig = &acl.MitmConfig{DetailedHttpLogs: true}
	pctx.Req.Header.Set("Authorization", "secret")
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	sctx.dialAttrs = dialContextLoggerFields(pctx, sctx, conn, 3*time.Millisecond)
	HTTPErrorHandler(failedResponseWriter{}, pctx, errors.New("upstream failed"))
	rejectResponse(pctx, tunnelLimitError{errors.New("limit reached")})
	var statuses []float64
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		require.Equal(t, "pipe", record[LogFieldOutLocalAddr])
		require.Equal(t, "pipe", record[LogFieldOutRemoteAddr])
		require.Equal(t, float64(3), record[LogFieldConnEstablishMS])
		require.Equal(t, "http://example.com/", record[LogMitmReqUrl])
		require.Equal(t, "GET", record[LogMitmReqMethod])
		require.Contains(t, string(line), "[REDACTED]")
		require.NotContains(t, string(line), "secret")
		if record["level"] == "ERROR" {
			require.Equal(t, 1, bytes.Count(line, []byte(`"status_code":`)))
			statuses = append(statuses, record["status_code"].(float64))
		}
	}
	require.Equal(t, []float64{500, 500, 500, 429}, statuses)
}
