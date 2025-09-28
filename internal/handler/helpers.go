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

func DecompressBody(body []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	decompressedData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
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
