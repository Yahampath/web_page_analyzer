package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type ctxKeyRequestID struct{}

func RequestIDLoggerMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	// configure global format once
	logger.SetFormatter(&log.TextFormatter{
		TimestampFormat: time.RFC3339,
		FullTimestamp:   true,
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	
			w.Header().Set(`Access-Control-Allow-Origin`, `*`)
			w.Header().Set(`Access-Control-Allow-Methods`, `POST, GET, OPTIONS`)
			w.Header().Set(`Access-Control-Allow-Headers`, `Content-Type, x-request-id`)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			reqID := r.Header.Get(`x-request-id`)
			if reqID == "" {
				reqID = uuid.NewString()
			}

			w.Header().Set(`x-request-id`, reqID)
			ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, reqID)
			srw := &requestIdStatusRecorder{ResponseWriter: w, status: http.StatusOK}

			start := time.Now()
			defer func() {
				duration := time.Since(start)
				entry := logger.WithFields(log.Fields{
					`timestamp`:  time.Now().Format(time.RFC3339),
					`method`:     r.Method,
					`path`:       r.URL.Path,
					`status`:     srw.status,
					`request_id`: reqID,
					`duration`:   duration.String(),
				})

				if rec := recover(); rec != nil {
					// panic: log stack + return JSON error
					entry = entry.WithFields(log.Fields{
						`error`: fmt.Sprintf(`%v`, rec),
						`stack`: string(debug.Stack()),
					})
					entry.Error(`panic recovered`)
					srw.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(srw).Encode(map[string]string{
						`error`:      `internal server error`,
						`request_id`: reqID,
					})
				} else if srw.status >= 400 {
					entry.Error(`request completed with error status`)
				} else {
					entry.Info(`request completed`)
				}
			}()

			next.ServeHTTP(srw, r.WithContext(ctx))
		})
	}
}

// statusRecorder captures HTTP status codes
type requestIdStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *requestIdStatusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
