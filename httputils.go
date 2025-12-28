package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Default values for tests or external calls
var (
	defaultHTTPClient = &http.Client{}
	defaultUserAgent  = "go-plh-client/1.0"
	defaultCCPURL     = "https://esi.evetech.net/latest"
	defaultZKillURL   = "https://zkillboard.com/api"
)

// fetchURL performs an HTTP request using the provided client and userAgent.
// If client or userAgent is nil/empty, defaults are used.
func fetchURL(ctx context.Context, client *http.Client, method, url string, params map[string]string, body io.Reader, userAgent string) ([]byte, error) {
	if client == nil {
		client = defaultHTTPClient
	}
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

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

// Wrapper functions for CCP and zKill APIs
func ccpGet(ctx context.Context, url string, params map[string]string) ([]byte, error) {
	return fetchURL(ctx, httpClient, http.MethodGet, defaultCCPURL+url, params, nil, userAgent)
}

func ccpPost(ctx context.Context, url string, params map[string]string, body io.Reader) ([]byte, error) {
	return fetchURL(ctx, httpClient, http.MethodPost, defaultCCPURL+url, params, body, userAgent)
}

func zkillGet(ctx context.Context, url string) ([]byte, error) {
	return fetchURL(ctx, httpClient, http.MethodGet, defaultZKillURL+url, nil, nil, userAgent)
}
