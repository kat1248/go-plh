package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func fetchURL(ctx context.Context, client *http.Client, method, url string, params map[string]string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", userAgent)

	if len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d - %s", resp.StatusCode, url)
	}

	return respBody, nil
}

func ccpGet(ctx context.Context, client *http.Client, url string, params map[string]string) ([]byte, error) {
	return fetchURL(ctx, client, http.MethodGet, ccpEsiURL+url, params, nil)
}

func ccpPost(ctx context.Context, client *http.Client, url string, params map[string]string, body io.Reader) ([]byte, error) {
	return fetchURL(ctx, client, http.MethodPost, ccpEsiURL+url, params, body)
}

func zkillGet(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	return fetchURL(ctx, client, http.MethodGet, zkillAPIURL+url, nil, nil)
}

func zkillCheck(ctx context.Context, client *http.Client) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zkillAPIURL, nil)
	if err != nil {
		return false
	}
	req.Header.Add("User-Agent", userAgent)

	// temporarily turn off retries
	// retries := client.MaxRetries
	// client.MaxRetries = 0
	// defer func() {
	// 	client.MaxRetries = retries
	// }()
	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return false
	}

	return true
}
