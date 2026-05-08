package service

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "strings"
    "time"

    "github.com/google/generative-ai-go/genai"
    "google.golang.org/api/option"
)

func (s *articleService) GenerateTags(ctx context.Context, content string) (*json.RawMessage, error) {
    var lastErr error

    for attempt := 0; attempt < 6; attempt++ {
        if attempt > 0 {
            waitTime := time.Duration(1<<uint(attempt-1)) * time.Second
            log.Printf("Gemini quota error, retrying in %v... (attempt %d/6)", waitTime, attempt+1)
            select {
            case <-time.After(waitTime):
            case <-ctx.Done():
                return fallbackTags(content), ctx.Err()
            }
        }

        client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
        if err != nil {
            return fallbackTags(content), err
        }
        defer client.Close()

        modelName := os.Getenv("GEMINI_MODEL")
        if modelName == "" {
            modelName = "gemini-2.5-flash-lite"
        }
        model := client.GenerativeModel(modelName)

        prompt := genai.Text(fmt.Sprintf(
            "以下の記事本文から、技術タグを3〜5個抽出してください。\n"+
                "出力はJSON配列のみ（例: [\"Go\",\"RAG\"]）。説明文や前置きは不要です。\n\n"+
                "記事本文:\n%s",
            content,
        ))

        resp, err := model.GenerateContent(ctx, prompt)
        if err != nil {
            if isQuotaError(err) {
                lastErr = err
                if attempt < 5 {
                    continue
                }
                log.Printf("Gemini quota exceeded after retries (max ~60s), using fallback tags: %v", err)
                return fallbackTags(content), nil
            }
            return fallbackTags(content), err
        }

        if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
            generated := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
            normalized, parseErr := normalizeTags(generated)
            if parseErr == nil {
                return normalized, nil
            }
            log.Printf("normalizeTags parse error, using fallback tags: %v", parseErr)
        }

        return fallbackTags(content), nil
    }

    if lastErr != nil {
        log.Printf("GenerateTags failed after retries: %v", lastErr)
    }
    return fallbackTags(content), nil
}

func normalizeTags(raw string) (*json.RawMessage, error) {
    cleaned := strings.TrimSpace(raw)
    if cleaned == "" {
        return tagsToRawMessage([]string{}), nil
    }

    start := strings.Index(cleaned, "[")
    end := strings.LastIndex(cleaned, "]")
    if start >= 0 && end > start {
        cleaned = cleaned[start : end+1]
    }

    var tags []string
    if err := json.Unmarshal([]byte(cleaned), &tags); err != nil {
        return nil, err
    }

    return tagsToRawMessage(normalizeTagList(tags)), nil
}

func fallbackTags(content string) *json.RawMessage {
    lower := strings.ToLower(content)
    candidates := []struct {
        tag     string
        keyword string
    }{
        {tag: "Go", keyword: "go"},
        {tag: "React", keyword: "react"},
        {tag: "TypeScript", keyword: "typescript"},
        {tag: "Gemini", keyword: "gemini"},
        {tag: "LLM", keyword: "llm"},
        {tag: "RAG", keyword: "rag"},
        {tag: "Supabase", keyword: "supabase"},
        {tag: "Docker", keyword: "docker"},
        {tag: "Kubernetes", keyword: "kubernetes"},
        {tag: "CI", keyword: "ci"},
        {tag: "Testing", keyword: "test"},
    }

    tags := make([]string, 0, 5)
    for _, c := range candidates {
        if strings.Contains(lower, c.keyword) {
            tags = append(tags, c.tag)
        }
        if len(tags) >= 5 {
            break
        }
    }

    return tagsToRawMessage(normalizeTagList(tags))
}

func normalizeTagList(tags []string) []string {
    uniq := make(map[string]struct{}, len(tags))
    result := make([]string, 0, len(tags))

    for _, tag := range tags {
        trimmed := strings.TrimSpace(tag)
        if trimmed == "" {
            continue
        }
        if _, exists := uniq[trimmed]; exists {
            continue
        }
        uniq[trimmed] = struct{}{}
        result = append(result, trimmed)
        if len(result) >= 5 {
            break
        }
    }

    return result
}

func tagsToRawMessage(tags []string) *json.RawMessage {
    b, err := json.Marshal(tags)
    if err != nil {
        empty := json.RawMessage("[]")
        return &empty
    }
    raw := json.RawMessage(b)
    return &raw
}

func hasNonEmptyTags(tags *json.RawMessage) bool {
    if tags == nil {
        return false
    }

    var parsed []string
    if err := json.Unmarshal(*tags, &parsed); err != nil {
        return strings.TrimSpace(string(*tags)) != ""
    }

    return len(normalizeTagList(parsed)) > 0
}
