package adaptors

import "context"

type WebClient interface {
	Do(ctx context.Context, url string, method string) ([]byte, int, error)
}