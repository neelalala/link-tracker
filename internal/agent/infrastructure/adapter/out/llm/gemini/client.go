package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	baseApiUrl  = "https://generativelanguage.googleapis.com/v1beta"
	model3Flash = "gemini-3-flash-preview"
	method      = "generateContent"
)

type Client struct {
	httpClient *http.Client
	apiURL     string
	key        string
	timeout    time.Duration

	log *slog.Logger
}

func New(httpClient *http.Client, apiKey string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		httpClient: httpClient,
		apiURL:     fmt.Sprintf("%s/models/%s:%s", baseApiUrl, model3Flash, method),
		key:        apiKey,
		timeout:    timeout,
		log:        log,
	}
}

type geminiRequest struct {
	Contents []content `json:"contents"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type geminiErrorResponse struct {
	Error struct {
		Message string `json:"messages"`
	} `json:"error"`
}

func (c *Client) GenerateText(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	reqBody := geminiRequest{
		Contents: []content{
			{
				Parts: []part{
					{
						Text: prompt,
					},
				},
			},
		},
	}

	c.log.Info("Sending request to generation", "prompt", prompt)

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("couldn't serialize request: %w", err)
	}

	c.log.Debug("Request body", "json", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("couldn't create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.key)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("couldn't read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr geminiErrorResponse
		if err := json.Unmarshal(bodyBytes, &apiErr); err == nil {
			return "", fmt.Errorf("gemini api returned error (code %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("gemini api returned code %d, body = %s", resp.StatusCode, string(bodyBytes))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", fmt.Errorf("error unmarshaling gemini response: %w", err)
	}

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		return geminiResp.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", errors.New("got empty response from gemini api")
}
