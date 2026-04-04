package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const maxRetries = 3

// fetchURL sends an HTTP request built from method, url, params, and body using the provided context and client and returns the response body bytes.
// It sets the "Accept: application/json" and "User-Agent" headers, appends any entries in params as URL query parameters, and treats only HTTP 200 as a successful response; any other status code results in an error.
// On HTTP 429 Too Many Requests, it respects the Retry-After header (defaulting to 5s) and retries up to maxRetries times.
func fetchURL(ctx context.Context, client *http.Client, method, url string, params map[string]string, body io.Reader) ([]byte, error) {
	for attempt := range maxRetries {
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

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()

			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("http error %d - %s", resp.StatusCode, url)
			}

			delay := 5 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
					delay = time.Duration(secs) * time.Second
				}
			}

			log.WithFields(log.Fields{
				"url":     url,
				"delay":   delay,
				"attempt": attempt + 1,
			}).Warn("rate limited, retrying after delay")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			continue
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

	return nil, fmt.Errorf("max retries exceeded - %s", url)
}

// ccpGet fetches the specified path from the CCP ESI API using HTTP GET and returns the response body.
// If params is non-empty, its key/value pairs are appended as URL query parameters.
// The returned error is non-nil on request/response failures, including when the response status is not 200 OK.
func ccpGet(ctx context.Context, client *http.Client, url string, params map[string]string) ([]byte, error) {
	return fetchURL(ctx, client, http.MethodGet, ccpEsiURL+url, params, nil)
}

// ccpPost posts the provided body to the CCP ESI API at the given path.
// It returns the response body bytes on success, or an error if request creation, execution, response reading fails, or if the response status is not 200 OK.
func ccpPost(ctx context.Context, client *http.Client, url string, params map[string]string, body io.Reader) ([]byte, error) {
	return fetchURL(ctx, client, http.MethodPost, ccpEsiURL+url, params, body)
}

// zkillGet fetches the resource at the given path from the zkill API and returns the response body.
//
// It returns the response body bytes on success, or a non-nil error if the request failed or the
// response status was not 200 OK.
func zkillGet(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	return fetchURL(ctx, client, http.MethodGet, zkillAPIURL+url, nil, nil)
}

// zkillCheck verifies availability of the zKillboard API endpoint.
// It issues a GET request to zkillAPIURL using the provided context and HTTP client with the User-Agent header set; it returns false if request creation or execution fails or if the server responds with HTTP 503 Service Unavailable, and returns true otherwise.
func zkillCheck(ctx context.Context, client *http.Client) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zkillAPIURL, nil)
	if err != nil {
		return false
	}
	req.Header.Add("User-Agent", userAgent)

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
