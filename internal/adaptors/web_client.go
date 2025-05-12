package adaptors

import (
	"context"
	"io"
	"net/http"
	"time"
	"web_page_analyzer/internal/pkg/errors"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"web_page_analyzer/internal/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

type WebClient struct {
	client *http.Client
	log    *log.Logger
}

func NewWebClient(timeout time.Duration, log *log.Logger) *WebClient {
	rTripper := promhttp.InstrumentRoundTripperDuration(
		 metrics.HTTPClientRequestDuration,
		 promhttp.InstrumentRoundTripperCounter(metrics.HTTPClientRequestsTotal, http.DefaultTransport))

	return &WebClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: rTripper,
		},
		log: log,
	}
}

func (w *WebClient) Do(ctx context.Context, url string, method string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		w.log.WithError(err).Error(`failed to create request`)
		return nil, 0, errors.Wrap(err, `failed to create request`)
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := w.client.Do(req)
	if err != nil {
		w.log.WithError(err).Error(`url is invalid`)
		return nil, 0, errors.Wrap(err, `url is invalid`)
	}
	defer resp.Body.Close()

	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		w.log.Errorf(`failed to read response body. error: %v`, err)
		return nil, 0, errors.Wrap(err, `failed to read response body`)
	}

	return bodyByte, resp.StatusCode, nil
}
