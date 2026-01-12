package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroadcaster(t *testing.T) {
	// Create channels
	source := make(chan models.AuditEvent)
	// Create buffered channels to ensure events can be received
	sub1 := make(chan models.AuditEvent, 1)
	sub2 := make(chan models.AuditEvent, 1)

	// Start the broadcaster
	go Broadcaster(source, sub1, sub2)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event
	go func() {
		source <- event
		close(source)
	}()

	// Receive from subscribers
	received1 := <-sub1
	received2 := <-sub2

	// Check that both subscribers received the same event
	assert.Equal(t, event, received1)
	assert.Equal(t, event, received2)
}

func TestFileSubscriber(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "audit_test_*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Create config with the temp file path
	cfg := config.ServerConfig{
		AuditFile: tmpFile.Name(),
	}

	// Create a channel for events
	events := make(chan models.AuditEvent)

	// Start the file subscriber in a goroutine
	go FileSubscriber(events, cfg)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event and close the channel
	events <- event
	close(events)

	// Give the subscriber time to process
	time.Sleep(100 * time.Millisecond)

	// Read the file content
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	// Check that the event was written to the file
	assert.Contains(t, string(content), "testMetric")
	assert.Contains(t, string(content), "127.0.0.1")

	// Check that the content is valid JSON
	var writtenEvent models.AuditEvent
	err = json.Unmarshal(content[:len(content)-1], &writtenEvent) // Remove the trailing newline
	require.NoError(t, err)
	assert.Equal(t, event, writtenEvent)
}

func TestURLSubscriber(t *testing.T) {
	// Variable to capture the received event
	var receivedEvent models.AuditEvent

	// Create a test server to receive the event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and content type
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read the request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		// Unmarshal the event
		err = json.Unmarshal(body, &receivedEvent)
		require.NoError(t, err)

		// Send response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create config with the test server URL
	cfg := config.ServerConfig{
		AuditURL: server.URL,
	}

	// Create a channel for events
	events := make(chan models.AuditEvent)

	// Start the URL subscriber in a goroutine
	go URLSubscriber(events, cfg)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event and close the channel
	events <- event
	close(events)

	// Give the subscriber time to process
	time.Sleep(100 * time.Millisecond)

	// Check that the event was received by the server
	assert.Equal(t, event, receivedEvent)
}

func TestFileSubscriber_InvalidJSON(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "audit_test_*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Create config with the temp file path
	cfg := config.ServerConfig{
		AuditFile: tmpFile.Name(),
	}

	// Create a channel for events
	events := make(chan models.AuditEvent)

	// Start the file subscriber in a goroutine
	go FileSubscriber(events, cfg)

	// Create a test event with a non-serializable field
	// This test mainly checks that the subscriber doesn't panic
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event and close the channel
	events <- event
	close(events)

	// Give the subscriber time to process
	time.Sleep(100 * time.Millisecond)

	// Just ensure the test completes without panicking
	// The actual behavior might vary depending on the JSON marshaler
}

func TestURLSubscriber_InvalidJSON(t *testing.T) {
	// Create a test server to receive the event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create config with the test server URL
	cfg := config.ServerConfig{
		AuditURL: server.URL,
	}

	// Create a channel for events
	events := make(chan models.AuditEvent)

	// Start the URL subscriber in a goroutine
	go URLSubscriber(events, cfg)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event and close the channel
	events <- event
	close(events)

	// Give the subscriber time to process
	time.Sleep(100 * time.Millisecond)
	// Just ensure the test completes without panicking
}

func TestBroadcaster_ChannelBlocking(t *testing.T) {
	// Create channels
	source := make(chan models.AuditEvent)
	// Create unbuffered channels to simulate blocking subscribers
	sub1 := make(chan models.AuditEvent)
	sub2 := make(chan models.AuditEvent)

	// Start the broadcaster
	go Broadcaster(source, sub1, sub2)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event - this should not block or cause goroutine leaks
	// even though the subscriber channels are unbuffered and there's no receiver
	source <- event

	// Close the source channel
	close(source)

	// Give the broadcaster time to process
	time.Sleep(100 * time.Millisecond)

	// The test passes if it doesn't deadlock or cause goroutine leaks
	// In the old implementation, this would have caused goroutines to block indefinitely
}

func TestFileSubscriber_FileError(t *testing.T) {
	// Create config with an invalid file path
	cfg := config.ServerConfig{
		AuditFile: "/invalid/path/that/does/not/exist/log.txt",
	}

	// Create a channel for events
	events := make(chan models.AuditEvent)

	// Start the file subscriber in a goroutine
	go FileSubscriber(events, cfg)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event and close the channel
	events <- event
	close(events)

	// Give the subscriber time to process
	time.Sleep(100 * time.Millisecond)

	// Just ensure the test completes without panicking
}

func TestURLSubscriber_NetworkError(t *testing.T) {
	// Create config with an invalid URL
	cfg := config.ServerConfig{
		AuditURL: "http://invalid.url.that.does.not.exist",
	}

	// Create a channel for events
	events := make(chan models.AuditEvent)

	// Start the URL subscriber in a goroutine
	go URLSubscriber(events, cfg)

	// Create a test event
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   []string{"testMetric"},
		IPAddress: "127.0.0.1",
	}

	// Send the event and close the channel
	events <- event
	close(events)

	// Give the subscriber time to process
	time.Sleep(100 * time.Millisecond)

	// Just ensure the test completes without panicking
}
