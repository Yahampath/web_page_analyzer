package adaptors

import (
    "context"
    "errors"
    "io"
    "net/http"
    "strings"
    "testing"
    "time"

    log "github.com/sirupsen/logrus"
)

// RoundTripFunc lets us mock http.RoundTripper easily.
type RoundTripFunc func(req *http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
    return f(req)
}

func TestWebClient_Do(t *testing.T) {
    logger := log.New()
    ctx := context.Background()
    const testURL = "http://example.com"

    cases := []struct {
        name       string
        setup      func() *WebClient
        wantBody   string
        wantCode   int
        wantErr    bool
    }{
        {
            name: "success",
            setup: func() *WebClient {
                // stub transport returns 200 + "OK"
                wc := &WebClient{
                    client: &http.Client{
                        Timeout: 1 * time.Second,
                        Transport: RoundTripFunc(func(req *http.Request) (*http.Response, error) {
                            return &http.Response{
                                StatusCode: 200,
                                Body:       io.NopCloser(strings.NewReader("OK")),
                                Header:     make(http.Header),
                            }, nil
                        }),
                    },
                    log: logger,
                }
                return wc
            },
            wantBody: "OK",
            wantCode: 200,
            wantErr:  false,
        },
        {
            name: "network error",
            setup: func() *WebClient {
                // transport returns an error
                wc := &WebClient{
                    client: &http.Client{
                        Timeout: 1 * time.Second,
                        Transport: RoundTripFunc(func(req *http.Request) (*http.Response, error) {
                            return nil, errors.New("network failure")
                        }),
                    },
                    log: logger,
                }
                return wc
            },
            wantBody: "",
            wantCode: 0,
            wantErr:  true,
        },
        {
            name: "invalid URL",
            setup: func() *WebClient {
                // normal client, but URL is invalid => NewRequestWithContext fails
                return NewWebClient(1*time.Second, logger)
            },
            wantBody: "",
            wantCode: 0,
            wantErr:  true,
        },
        {
            name: "read body error",
            setup: func() *WebClient {
                // transport returns a response whose Body.Read always errors
                wc := &WebClient{
                    client: &http.Client{
                        Timeout: 1 * time.Second,
                        Transport: RoundTripFunc(func(req *http.Request) (*http.Response, error) {
                            return &http.Response{
                                StatusCode: 200,
                                Body:       errReadCloser{},
                                Header:     make(http.Header),
                            }, nil
                        }),
                    },
                    log: logger,
                }
                return wc
            },
            wantBody: "",
            wantCode: 0,
            wantErr:  true,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            wc := tc.setup()
            body, code, err := wc.Do(ctx, testURL, http.MethodGet)

            if tc.wantErr {
                if err == nil {
                    t.Fatal("expected error, got nil")
                }
            } else {
                if err != nil {
                    t.Fatalf("unexpected error: %v", err)
                }
            }

            if got := string(body); got != tc.wantBody {
                t.Errorf("body = %q; want %q", got, tc.wantBody)
            }
            if code != tc.wantCode {
                t.Errorf("code = %d; want %d", code, tc.wantCode)
            }
        })
    }
}

// errReadCloser is an io.ReadCloser that always errors on Read.
type errReadCloser struct{}

func (e errReadCloser) Read(p []byte) (int, error) {
    return 0, errors.New("read failed")
}
func (e errReadCloser) Close() error {
    return nil
}