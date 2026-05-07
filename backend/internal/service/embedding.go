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

    // 初期的に使うモデル名（過去の設定）。見つからない場合は ListModels を参照して再試行する。
    modelName := "models/text-embedding-004"

    url := func(m string) string {
        return "https://generativelanguage.googleapis.com/v1beta/models/" + m + ":embedContent?key=" + apiKey
    }(modelName)

    payload := map[string]interface{}{
        "model": modelName,
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
        // 404 の場合は ListModels で埋め込み可能なモデルを探して再試行
        if resp.StatusCode == http.StatusNotFound {
            log.Printf("embedding model not found (%s). attempting to discover embedding-capable models...", modelName)
            candidate, listErr := discoverEmbeddingModel(ctx, apiKey)
            if listErr == nil && candidate != "" {
                // 再試行
                url = "https://generativelanguage.googleapis.com/v1beta/models/" + candidate + ":embedContent?key=" + apiKey
                payload["model"] = candidate
                body2, _ := json.Marshal(payload)
                req2, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body2)))
                req2.Header.Set("Content-Type", "application/json")
                resp2, err2 := (&http.Client{}).Do(req2)
                if err2 != nil {
                    return nil, fmt.Errorf("failed to make retry request: %w", err2)
                }
                defer resp2.Body.Close()
                respBody2, err := io.ReadAll(resp2.Body)
                if err != nil {
                    return nil, fmt.Errorf("failed to read retry response: %w", err)
                }
                if resp2.StatusCode != http.StatusOK {
                    return nil, fmt.Errorf("API error (%d): %s", resp2.StatusCode, string(respBody2))
                }
                var result2 struct {
                    Embedding struct {
                        Values []float64 `json:"values"`
                    } `json:"embedding"`
                }
                if err := json.Unmarshal(respBody2, &result2); err != nil {
                    return nil, fmt.Errorf("failed to parse retry response: %w", err)
                }
                return result2.Embedding.Values, nil
            }
        }
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

// discoverEmbeddingModel queries ListModels and returns a candidate model name
// that likely supports embedding. It is a best-effort heuristic.
func discoverEmbeddingModel(ctx context.Context, apiKey string) (string, error) {
    url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }
    resp, err := (&http.Client{}).Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("ListModels API error (%d): %s", resp.StatusCode, string(body))
    }

    var parsed map[string]interface{}
    if err := json.Unmarshal(body, &parsed); err != nil {
        return "", err
    }

    modelsRaw, ok := parsed["models"]
    if !ok {
        return "", fmt.Errorf("no models field in ListModels response")
    }
    models, ok := modelsRaw.([]interface{})
    if !ok {
        return "", fmt.Errorf("unexpected models format")
    }

    // Heuristic: prefer model names containing "embed" or "embedding"
    for _, m := range models {
        mp, ok := m.(map[string]interface{})
        if !ok {
            continue
        }
        nameRaw, ok := mp["name"]
        if !ok {
            continue
        }
        name, ok := nameRaw.(string)
        if !ok {
            continue
        }
        low := strings.ToLower(name)
        if strings.Contains(low, "embed") || strings.Contains(low, "embedding") {
            return name, nil
        }
        // also check metadata.supportedMethods if present
        if metaRaw, ok := mp["metadata"]; ok {
            if meta, ok := metaRaw.(map[string]interface{}); ok {
                if methodsRaw, ok := meta["supportedMethods"]; ok {
                    if methods, ok := methodsRaw.([]interface{}); ok {
                        for _, mm := range methods {
                            if ms, ok := mm.(string); ok {
                                if strings.Contains(strings.ToLower(ms), "embed") {
                                    return name, nil
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    return "", fmt.Errorf("no embedding-capable model found in ListModels response")
}
