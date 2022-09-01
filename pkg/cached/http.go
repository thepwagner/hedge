package cached

import (
	"context"
	"io"
	"net/http"
)

func URLFetcher(client *http.Client) Function[string, []byte] {
	return func(ctx context.Context, u string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
}
