package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func (s *articleService) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
    var lastErr error

    for attempt := 0; attempt < 6; attempt++ {
        if attempt > 0 {
            waitTime := time.Duration(1<<uint(attempt-1)) * time.Second
            log.Printf("Gemini quota error, retrying embedding in %v... (attempt %d/6)", waitTime, attempt+1)
            select {
            case <-time.After(waitTime):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }

        embedding, err := s.generateEmbeddingViaAPI(ctx, text)
        if err != nil {
            if isQuotaError(err) {
                lastErr = err
                if attempt < 5 {
                    continue
                }
                log.Printf("Gemini quota exceeded for embedding after retries, returning nil: %v", err)
                return nil, nil
            }
            return nil, err
        }

        return embedding, nil
    }

    if lastErr != nil {
        log.Printf("GenerateEmbedding failed after retries: %v", lastErr)
    }
    return nil, nil
}

func (s *articleService) generateEmbeddingViaAPI(ctx context.Context, text string) ([]float64, error) {
    apiKey := os.Getenv("GEMINI_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("GEMINI_API_KEY not set")
    }

    url := "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=" + apiKey

    payload := map[string]interface{}{
        "model": "models/text-embedding-004",
        "content": map[string]interface{}{
            "parts": []map[string]string{{"text": text}},
        },
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        errMsg := string(respBody)
        if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
            return nil, fmt.Errorf("quota exceeded: %s", errMsg)
        }
        return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
    }

    var result struct {
        Embedding struct {
            Values []float64 `json:"values"`
        } `json:"embedding"`
    }

    if err := json.Unmarshal(respBody, &result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return result.Embedding.Values, nil
}
