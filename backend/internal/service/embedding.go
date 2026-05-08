package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	embeddingModelDiscoverySuccessTTL = 30 * time.Minute
	embeddingModelDiscoveryFailureTTL = 5 * time.Minute
)

type embeddingModelCacheEntry struct {
	modelName string
	expiresAt time.Time
}

var (
	embeddingModelCacheMu sync.RWMutex
	embeddingModelCache   = map[string]embeddingModelCacheEntry{}
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

	// SDK の model 指定に合わせて、環境変数で上書き可能にする
	modelName := os.Getenv("EMBEDDING_MODEL")
	if modelName == "" {
		modelName = "models/gemini-embedding-001"
	}

	apiVersions := []string{"v1", "v1beta"}
	lastStatusCode := 0
	lastErrMsg := ""

	for _, apiVersion := range apiVersions {
		embedding, statusCode, errMsg, err := requestEmbedding(ctx, apiKey, modelName, text, apiVersion)
		if err == nil {
			return embedding, nil
		}

		lastStatusCode = statusCode
		lastErrMsg = errMsg

		if statusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("quota exceeded: %s", errMsg)
		}

		if statusCode != http.StatusNotFound {
			return nil, fmt.Errorf("API error (%d): %s", statusCode, errMsg)
		}

		log.Printf("embedding model not found (%s on %s). attempting to discover embedding-capable models...", modelName, apiVersion)
	}

	for _, apiVersion := range apiVersions {
		if cachedModel, cacheHit := getCachedEmbeddingModel(apiVersion); cacheHit {
			if cachedModel != "" {
				embedding, statusCode, errMsg, err := requestEmbedding(ctx, apiKey, cachedModel, text, apiVersion)
				if err == nil {
					return embedding, nil
				}
				lastStatusCode = statusCode
				lastErrMsg = errMsg
				if statusCode != http.StatusNotFound {
					if statusCode == http.StatusTooManyRequests {
						return nil, fmt.Errorf("quota exceeded: %s", errMsg)
					}
					return nil, fmt.Errorf("API error (%d): %s", statusCode, errMsg)
				}
				clearCachedEmbeddingModel(apiVersion)
			}
			if cachedModel == "" {
				continue
			}
		}

		candidate, listErr := discoverEmbeddingModel(ctx, apiKey, apiVersion)
		if listErr != nil {
			log.Printf("discover embedding model failed on %s: %v", apiVersion, listErr)
			setCachedEmbeddingModel(apiVersion, "", embeddingModelDiscoveryFailureTTL)
			continue
		}
		if candidate == "" {
			setCachedEmbeddingModel(apiVersion, "", embeddingModelDiscoveryFailureTTL)
			continue
		}
		setCachedEmbeddingModel(apiVersion, candidate, embeddingModelDiscoverySuccessTTL)

		embedding, statusCode, errMsg, err := requestEmbedding(ctx, apiKey, candidate, text, apiVersion)
		if err == nil {
			log.Printf("embedding model discovered: %s (api=%s)", candidate, apiVersion)
			return embedding, nil
		}

		lastStatusCode = statusCode
		lastErrMsg = errMsg

		if statusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("quota exceeded: %s", errMsg)
		}

		if statusCode != http.StatusNotFound {
			return nil, fmt.Errorf("API error (%d): %s", statusCode, errMsg)
		}
	}

	if lastStatusCode == 0 {
		return nil, fmt.Errorf("failed to discover embedding model on both v1 and v1beta")
	}

	return nil, fmt.Errorf("API error (%d): %s", lastStatusCode, lastErrMsg)
}

func getCachedEmbeddingModel(apiVersion string) (string, bool) {
	embeddingModelCacheMu.RLock()
	entry, ok := embeddingModelCache[apiVersion]
	embeddingModelCacheMu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		clearCachedEmbeddingModel(apiVersion)
		return "", false
	}
	return entry.modelName, true
}

func setCachedEmbeddingModel(apiVersion, modelName string, ttl time.Duration) {
	embeddingModelCacheMu.Lock()
	embeddingModelCache[apiVersion] = embeddingModelCacheEntry{
		modelName: modelName,
		expiresAt: time.Now().Add(ttl),
	}
	embeddingModelCacheMu.Unlock()
}

func clearCachedEmbeddingModel(apiVersion string) {
	embeddingModelCacheMu.Lock()
	delete(embeddingModelCache, apiVersion)
	embeddingModelCacheMu.Unlock()
}

func requestEmbedding(ctx context.Context, apiKey, modelName, text, apiVersion string) ([]float64, int, string, error) {
	formattedModelName := strings.TrimPrefix(modelName, "models/")
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/%s/models/%s:embedContent?key=%s", apiVersion, formattedModelName, apiKey)

	outputDimensionality := 768
	if dimEnv := os.Getenv("EMBEDDING_DIM"); dimEnv != "" {
		if parsed, err := strconv.Atoi(dimEnv); err == nil && parsed > 0 {
			outputDimensionality = parsed
		}
	}

	payload := map[string]interface{}{
		"model": "models/" + formattedModelName,
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": text}},
		},
		"outputDimensionality": outputDimensionality,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, string(respBody), fmt.Errorf("non-200 response")
	}

	var result struct {
		Embedding struct {
			Values []float64 `json:"values"`
		} `json:"embedding"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, 0, "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Embedding.Values, http.StatusOK, "", nil
}

// discoverEmbeddingModel queries ListModels and returns a candidate model name
// that likely supports embedding. It is a best-effort heuristic.
func discoverEmbeddingModel(ctx context.Context, apiKey, apiVersion string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/%s/models?key=%s", apiVersion, apiKey)
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

		// 互換性のため、supportedMethods / supportedGenerationMethods の両方を見る
		if methodNameIncludesEmbed(mp["supportedMethods"]) || methodNameIncludesEmbed(mp["supportedGenerationMethods"]) {
			return name, nil
		}
	}

	return "", fmt.Errorf("no embedding-capable model found in ListModels response")
}

func methodNameIncludesEmbed(raw interface{}) bool {
	methods, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, m := range methods {
		ms, ok := m.(string)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(ms), "embed") {
			return true
		}
	}
	return false
}
