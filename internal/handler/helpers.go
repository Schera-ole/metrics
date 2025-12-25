package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	models "github.com/Schera-ole/metrics/internal/model"
)

func CalculatedHash(compressedBody []byte, key string) []byte {
	keyBytes := []byte(key)
	h := hmac.New(sha256.New, keyBytes)
	h.Write(compressedBody)
	return h.Sum(nil)
}

func VerifyRequestHash(body []byte, headerHash string, key string) error {
	if key == "" || headerHash == "" {
		return nil
	}
	calculatedHash := CalculatedHash(body, key)
	headerHashBytes, err := hex.DecodeString(headerHash)
	if err != nil {
		return fmt.Errorf("invalid hash format")
	}
	if !bytes.Equal(headerHashBytes, calculatedHash) {
		return fmt.Errorf("hash mismatch")
	}
	return nil
}

var gzipReaderPool = sync.Pool{
	New: func() interface{} {
		// Create a temporary buffer to initialize the reader
		reader, err := gzip.NewReader(bytes.NewReader([]byte{}))
		if err != nil {
			// This should not happen with an empty buffer, but if it does, return nil
			return nil
		}
		return reader
	},
}

func DecompressBody(body []byte) ([]byte, error) {
	reader := gzipReaderPool.Get()
	if reader == nil {
		// If we couldn't get a reader from the pool, create a new one
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()
		decompressedData, err := io.ReadAll(gr)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress data: %w", err)
		}
		return decompressedData, nil
	}

	gr, ok := reader.(*gzip.Reader)
	if !ok {
		// If the type assertion fails, create a new reader
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()
		decompressedData, err := io.ReadAll(gr)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress data: %w", err)
		}
		return decompressedData, nil
	}

	err := gr.Reset(bytes.NewReader(body))
	if err != nil {
		// If resetting fails, create a new reader
		gr.Close()
		newGr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer newGr.Close()
		decompressedData, err := io.ReadAll(newGr)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress data: %w", err)
		}
		return decompressedData, nil
	}

	ok = false
	defer func() {
		gr.Close()
		if ok {
			gzipReaderPool.Put(gr)
		}
	}()

	decompressedData, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	ok = true
	return decompressedData, nil
}

func ReadRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	r.Body.Close()
	return body, nil
}

func SendAuditEvent(metrics []string, remoteAddr string, eventChan chan models.AuditEvent, logger *zap.SugaredLogger) {
	event := models.AuditEvent{
		TS:        time.Now().Format(time.RFC3339),
		Metrics:   metrics,
		IPAddress: remoteAddr,
	}
	select {
	case eventChan <- event:
		// Message was sent
	default:
		logger.Info("channel is full")
	}
}
