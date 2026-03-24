package bootstrap_test

import (
	"context"
	"io"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func newTestServer(t *testing.T) *infragin.Server {
	t.Helper()

	log := infralogger.NewNop()
	gin.SetMode(gin.TestMode)

	cfg := infragin.NewConfig("test", 0) // port 0 = random available port
	server := infragin.NewServer(cfg, log, func(e *gin.Engine) {
		e.GET("/health", func(c *gin.Context) {
			c.String(200, "ok")
		})
	})

	_ = server.StartAsync()

	return server
}

func TestShutdown_WithRealServer_NilOptionals(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	server := newTestServer(t)

	bg := bootstrap.BackgroundCancelsForTest{}
	err := bootstrap.ShutdownForTest(log, server, bg, syscall.SIGTERM)
	if err != nil {
		t.Errorf("expected nil error for clean shutdown, got %v", err)
	}
}

func TestShutdown_WithBackgroundCancels(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	server := newTestServer(t)

	_, cancel1 := context.WithCancel(context.Background())
	_, cancel2 := context.WithCancel(context.Background())
	_, cancel3 := context.WithCancel(context.Background())
	_, cancel4 := context.WithCancel(context.Background())
	_, cancel5 := context.WithCancel(context.Background())

	bg := bootstrap.NewBackgroundCancelsWithAll(cancel1, cancel2, cancel3, cancel4, cancel5)

	err := bootstrap.ShutdownForTest(log, server, bg, syscall.SIGTERM)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestShutdown_WithInterruptSignal(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	server := newTestServer(t)

	bg := bootstrap.BackgroundCancelsForTest{}
	err := bootstrap.ShutdownForTest(log, server, bg, syscall.SIGINT)
	if err != nil {
		t.Errorf("expected nil error for SIGINT shutdown, got %v", err)
	}
}

func TestShutdown_WithSSEBrokerAndLogService(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	server := newTestServer(t)

	// Create a real SSE broker to test its shutdown path
	broker := bootstrap.SetupSSEForTest(&bootstrap.CommandDeps{
		Logger: log,
		Config: &mockConfig{},
	})

	mockLogSvc := &mockLogService{}

	_, cancel1 := context.WithCancel(context.Background())
	_, cancel2 := context.WithCancel(context.Background())
	_, cancel3 := context.WithCancel(context.Background())
	_, cancel4 := context.WithCancel(context.Background())
	_, cancel5 := context.WithCancel(context.Background())
	bg := bootstrap.NewBackgroundCancelsWithAll(cancel1, cancel2, cancel3, cancel4, cancel5)

	err := bootstrap.ShutdownFullForTest(log, server, broker, mockLogSvc, nil, bg, syscall.SIGTERM)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !mockLogSvc.closed {
		t.Error("expected log service Close to be called during shutdown")
	}
}

func TestRunUntilInterrupt_ErrorChannel(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	errChan := make(chan error, 1)
	errChan <- io.ErrUnexpectedEOF

	bg := bootstrap.BackgroundCancelsForTest{}
	err := bootstrap.RunUntilInterruptForTest(log, nil, bg, errChan)
	if err == nil {
		t.Fatal("expected error from error channel")
	}
	if err.Error() != "server error: unexpected EOF" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// mockLogService implements logs.Service for testing.
type mockLogService struct {
	closed bool
}

func (m *mockLogService) StartCapture(_ context.Context, _, _ string, _ int) (logs.Writer, error) {
	return nil, nil //nolint:nilnil // test mock
}

func (m *mockLogService) StopCapture(_ context.Context, _, _ string) (*logs.LogMetadata, error) {
	return nil, nil //nolint:nilnil // test mock
}

func (m *mockLogService) GetLogReader(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil //nolint:nilnil // test mock
}

func (m *mockLogService) IsCapturing(_ string) bool {
	return false
}

func (m *mockLogService) GetLiveBuffer(_ string) logs.Buffer {
	return nil
}

func (m *mockLogService) Close() error {
	m.closed = true
	return nil
}
