// Copyright 2025 İrem Kuyucu
// Copyright 2025 Laurynas Četyrkinas
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	cookie     string
	httpClient *http.Client
	maxRetries int
}

func newClient(baseURL, cookie string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		cookie:  cookie,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: 3,
	}
}

func (c *Client) post(path string, body interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	return c.doWithRetry("POST", path, jsonData)
}

func (c *Client) get(path string) (*http.Response, error) {
	return c.doWithRetry("GET", path, nil)
}

func (c *Client) doWithRetry(method, path string, body []byte) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}

		var req *http.Request
		var err error

		if body != nil {
			req, err = http.NewRequest(method, c.baseURL+path, bytes.NewReader(body))
		} else {
			req, err = http.NewRequest(method, c.baseURL+path, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		if method == "POST" {
			req.Header.Set("Content-Type", "application/json")
		}

		if c.cookie != "" {
			req.AddCookie(&http.Cookie{
				Name:  "customer.account.session.token",
				Value: c.cookie,
			})
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue // Retry on network errors
		}

		// Success or non-retryable error
		if resp.StatusCode < 500 {
			return resp, nil
		}

		// 5xx error - read and close body, then retry
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func unmarshalJSON(resp *http.Response, v interface{}) error {
	data, err := readBody(resp)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
