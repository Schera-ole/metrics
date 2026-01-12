// Package audit provides audit logging functionality for the metrics server.
//
// It implements a publish-subscribe pattern for distributing audit events to
// multiple destinations including files and HTTP endpoints.
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Schera-ole/metrics/internal/config"
	models "github.com/Schera-ole/metrics/internal/model"
)

// AuditLogger is an interface for logging audit events.
type AuditLogger interface {
	// Log sends an audit event with the specified metrics, IP address, and timestamp.
	Log(metrics []string, ipAddress string)
}

// auditLogger is a concrete implementation of AuditLogger that sends events to a channel.
type auditLogger struct {
	eventChan chan models.AuditEvent
}

// NewAuditLogger creates a new AuditLogger that sends events to the provided channel.
func NewAuditLogger(eventChan chan models.AuditEvent) AuditLogger {
	return &auditLogger{
		eventChan: eventChan,
	}
}

// Log sends an audit event with the specified metrics and IP address.
func (a *auditLogger) Log(metrics []string, ipAddress string) {
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   metrics,
		IPAddress: ipAddress,
	}

	select {
	case a.eventChan <- event:
		// Event sent successfully
	default:
		// Channel is full, drop the event to prevent blocking
		fmt.Printf("AuditLogger: dropped event, channel is full\n")
	}
}

type Subscriber struct {
	ID int
}

// Broadcaster distributes audit events to multiple subscriber channels.
//
// It receives events from a source channel and sends them to all provided subscriber channels
// using select with default case to prevent blocking and goroutine leaks.
func Broadcaster(source <-chan models.AuditEvent, subs ...chan<- models.AuditEvent) {
	for evt := range source {
		for _, subChan := range subs {
			select {
			case subChan <- evt:
				// Event sent successfully
			default:
				// Channel is blocked, discard event to prevent goroutine leak
				fmt.Printf("Broadcaster: dropped event for blocked subscriber channel\n")
			}
		}
	}
}

// FileSubscriber writes audit events to a file.
func FileSubscriber(events <-chan models.AuditEvent, config config.ServerConfig) {
	for evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			fmt.Printf("FileSubscriber: ошибка маршалинга JSON: %v\n", err)
			continue
		}
		f, err := os.OpenFile(config.AuditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("FileSubscriber: не удалось открыть файл %s: %v\n", config.AuditFile, err)
			continue
		}
		_, err = f.WriteString(string(data) + "\n")
		if err != nil {
			fmt.Printf("FileSubscriber: ошибка записи в файл: %v\n", err)
		}
		f.Close()
		fmt.Printf("FileSubscriber: событие записано в файл: %s\n", string(data))
	}
}

// URLSubscriber sends audit events to an HTTP endpoint.
func URLSubscriber(events <-chan models.AuditEvent, config config.ServerConfig) {
	for evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			fmt.Printf("URLSubscriber: ошибка маршалинга JSON: %v\n", err)
			continue
		}
		resp, err := http.Post(config.AuditURL, "application/json", bytes.NewBuffer(data))
		if err != nil {
			fmt.Printf("URLSubscriber: ошибка отправки запроса на %s: %v\n", config.AuditURL, err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		fmt.Printf("URLSubscriber: событие отправлено по URL: %s\n", string(data))
	}
}
