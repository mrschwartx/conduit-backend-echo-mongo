package support

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type Response struct {
	Status int
	Header http.Header
	Raw    []byte
	Body   map[string]any
}

func (r Response) Value(path ...string) any {
	var current any = r.Body
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[key]
	}
	return current
}

func (r Response) String(path ...string) string {
	value, _ := r.Value(path...).(string)
	return value
}

func (r Response) Number(path ...string) float64 {
	value, _ := r.Value(path...).(float64)
	return value
}

func (r Response) Bool(path ...string) bool {
	value, _ := r.Value(path...).(bool)
	return value
}

func (r Response) Slice(path ...string) []any {
	value, _ := r.Value(path...).([]any)
	return value
}

func (r Response) Errors(field string) []string {
	messages := []string{}
	for _, entry := range r.Slice("errors", field) {
		if text, ok := entry.(string); ok {
			messages = append(messages, text)
		}
	}
	return messages
}

type Client struct {
	tb      testing.TB
	baseURL string
	http    *http.Client
}

func NewClient(tb testing.TB, baseURL string) *Client {
	tb.Helper()
	return &Client{tb: tb, baseURL: baseURL, http: &http.Client{}}
}

type RequestOptions struct {
	JSON        any
	RawBody     string
	ContentType string
	Token       string
	Headers     map[string]string
}

func (c *Client) Do(method, path string, options RequestOptions) Response {
	c.tb.Helper()

	var body io.Reader
	contentType := options.ContentType
	switch {
	case options.JSON != nil:
		encoded, err := json.Marshal(options.JSON)
		if err != nil {
			c.tb.Fatal(err)
		}
		body = bytes.NewReader(encoded)
		contentType = "application/json"
	case options.RawBody != "":
		body = strings.NewReader(options.RawBody)
	}

	request, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		c.tb.Fatal(err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if options.Token != "" {
		request.Header.Set("Authorization", "Token "+options.Token)
	}
	for key, value := range options.Headers {
		request.Header.Set(key, value)
	}

	response, err := c.http.Do(request)
	if err != nil {
		c.tb.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		c.tb.Fatal(err)
	}
	result := Response{Status: response.StatusCode, Header: response.Header, Raw: raw}
	var object map[string]any
	if json.Unmarshal(raw, &object) == nil {
		result.Body = object
	}
	return result
}

func (c *Client) Get(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodGet, path, firstOptions(options))
}

func (c *Client) Post(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodPost, path, firstOptions(options))
}

func (c *Client) Put(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodPut, path, firstOptions(options))
}

func (c *Client) Delete(path string, options ...RequestOptions) Response {
	return c.Do(http.MethodDelete, path, firstOptions(options))
}

func firstOptions(options []RequestOptions) RequestOptions {
	if len(options) == 0 {
		return RequestOptions{}
	}
	return options[0]
}
