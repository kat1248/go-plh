package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func fetchURL(method, url string, params map[string]string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", userAgent)

	if len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
			// fmt.Println(key, ":", value)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := localClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d - %s", resp.StatusCode, url)
	}

	return respBody, nil
}

func ccpGet(url string, params map[string]string) ([]byte, error) {
	return fetchURL("GET", ccpEsiURL+url, params, nil)
}

func ccpPost(url string, params map[string]string, body io.Reader) ([]byte, error) {
	return fetchURL("POST", ccpEsiURL+url, params, body)
}

func zkillGet(url string) ([]byte, error) {
	return fetchURL("GET", zkillAPIURL+url, nil, nil)
}

func zkillCheck() bool {
	req, err := http.NewRequest("GET", zkillURL, nil)
	if err != nil {
		return false
	}
	req.Header.Add("User-Agent", userAgent)

	// temporarily turn off retries
	retries := localClient.MaxRetries
	localClient.MaxRetries = 0
	defer func() {
		localClient.MaxRetries = retries
	}()
	resp, err := localClient.Do(req)
	if err != nil {
		return false
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return false
	}

	return true
}
