package middlewareinternal

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"compress/gzip"

	"go.uber.org/zap"
)

type (
	responseData struct {
		status int
		size   int
	}

	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

func LoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		logFn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}
			lw := loggingResponseWriter{
				ResponseWriter: w,
				responseData:   responseData,
			}
			uri := r.RequestURI
			method := r.Method

			next.ServeHTTP(&lw, r)
			duration := time.Since(start)

			logger.Infoln(
				"uri", uri,
				"method", method,
				"status", responseData.status,
				"duration", duration,
				"size", responseData.size,
			)

		}
		return http.HandlerFunc(logFn)
	}
}

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

type gzipWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gzw := gzipWriterPool.Get().(*gzip.Writer)
		gzw.Reset(w)
		defer func() {
			gzw.Close()
			gzipWriterPool.Put(gzw)
		}()
		gw := &gzipWriter{ResponseWriter: w, Writer: gzw}
		next.ServeHTTP(gw, r)
	})
}
