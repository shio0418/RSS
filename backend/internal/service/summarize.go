package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var bulletNumberRegex = regexp.MustCompile(`^\d+[.)]`)

func (s *articleService) Summarize(ctx context.Context, content string) (string, error) {
    var lastErr error

    for attempt := 0; attempt < 6; attempt++ {
        if attempt > 0 {
            waitTime := time.Duration(1<<uint(attempt-1)) * time.Second
            log.Printf("Gemini quota error, retrying in %v... (attempt %d/6)", waitTime, attempt+1)
            select {
            case <-time.After(waitTime):
            case <-ctx.Done():
                return fallbackSummary(content), ctx.Err()
            }
        }

        client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
        if err != nil {
            return fallbackSummary(content), err
        }
        defer client.Close()

        modelName := os.Getenv("GEMINI_MODEL")
        if modelName == "" {
            modelName = "gemini-2.5-flash-lite"
        }
        model := client.GenerativeModel(modelName)

        prompt := genai.Text(fmt.Sprintf(
            "以下の技術記事の内容を、エンジニアが30秒で理解できるように3つの箇条書きで要約してください。\n"+
                "前置き、あいさつ、補足説明は不要です。要約本文だけをそのまま出力してください。\n\n"+
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
                log.Printf("Gemini quota exceeded after retries (max ~60s), using fallback summary: %v", err)
                return fallbackSummary(content), nil
            }
            return fallbackSummary(content), err
        }

        if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
            return normalizeSummary(fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])), nil
        }
    }

    if lastErr != nil {
        log.Printf("Summarize failed after retries: %v", lastErr)
    }
    return fallbackSummary(content), nil
}

func normalizeSummary(summary string) string {
    cleaned := strings.TrimSpace(summary)
    if cleaned == "" {
        return cleaned
    }

    lines := strings.Split(cleaned, "\n")
    bulletLines := make([]string, 0, len(lines))
    seenBullet := false

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        if trimmed == "" {
            if seenBullet {
                bulletLines = append(bulletLines, "")
            }
            continue
        }

        lower := strings.ToLower(trimmed)
        if !seenBullet && (strings.HasPrefix(trimmed, "わかりました") ||
            strings.HasPrefix(trimmed, "承知しました") ||
            strings.HasPrefix(trimmed, "以下") ||
            strings.HasPrefix(trimmed, "要約")) {
            continue
        }

        if isBulletLine(trimmed) {
            seenBullet = true
            bulletLines = append(bulletLines, trimmed)
            continue
        }

        if seenBullet {
            bulletLines = append(bulletLines, trimmed)
            continue
        }

        if strings.Contains(lower, "要約") && strings.Contains(lower, "箇条書き") {
            continue
        }
    }

    if len(bulletLines) > 0 {
        return strings.TrimSpace(strings.Join(bulletLines, "\n"))
    }

    return cleaned
}

func isBulletLine(line string) bool {
    return strings.HasPrefix(line, "-") ||
        strings.HasPrefix(line, "・") ||
        bulletNumberRegex.MatchString(line)
}

func fallbackSummary(content string) string {
    cleaned := strings.TrimSpace(content)
    if cleaned == "" {
        return "要約を生成できませんでした"
    }

    runes := []rune(cleaned)
    if len(runes) > 180 {
        cleaned = string(runes[:180]) + "..."
    }

    return "要約を生成できなかったため、本文の冒頭を表示します: " + cleaned
}
