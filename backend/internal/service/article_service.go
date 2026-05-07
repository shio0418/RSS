package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/generative-ai-go/genai"
	"github.com/mmcdole/gofeed"
	"github.com/shio0418/RSS/internal/model"
	"github.com/shio0418/RSS/internal/repository"
	"google.golang.org/api/option"
)

var bulletNumberRegex = regexp.MustCompile(`^\d+[.)]`)

type ArticleService interface {
	FetchAndSummarize(ctx context.Context, urls []string) error
	ListArticles(ctx context.Context, limit int) ([]model.Article, error)
}

type articleService struct {
	repo repository.ArticleRepository
}

// コンストラクタ
func NewArticleService(repo repository.ArticleRepository) ArticleService {
	return &articleService{
		repo: repo,
	}
}

// 指定された複数のURLから記事を収集してDBに保存する、メインの司令塔
func (s *articleService) FetchAndSummarize(ctx context.Context, urls []string) error {
	jobs := make(chan string, len(urls))
	var wg sync.WaitGroup

	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobs {
				if err := s.FetchOneUrl(ctx, url); err != nil {
					fmt.Printf("Error fetching %s: %v\n", url, err)
				}
				time.Sleep(5 * time.Second)
			}
		}()
	}

	for _, url := range urls {
		jobs <- url
	}
	close(jobs)
	wg.Wait()
	return nil
}

func (s *articleService) FetchOneUrl(ctx context.Context, url string) error {
	fp := gofeed.NewParser()

	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return err
	}

	for _, item := range feed.Items {
		fmt.Printf("Attempting to save: %s | URL: %s\n", item.Title, item.Link)
		pubDate := time.Now()
		if item.PublishedParsed != nil {
			pubDate = *item.PublishedParsed
		}
		// 既にDBに要約が保存されている場合は Gemini を呼ばずにスキップする
		var content string
		var summary string
		var tags *json.RawMessage

		existing, err := s.repo.GetArticleByURL(ctx, item.Link)
		if err != nil {
			log.Printf("GetArticleByURL error: %v", err)
		}

		if existing != nil && existing.Summary != nil && *existing.Summary != "" {
			// 既存レコードの summary を使い、content は既存の content を流用
			summary = *existing.Summary
			content = existing.Content
			tags = existing.Tags
		} else {
			content, err = s.scrapeZennContent(item.Link)
			if err != nil {
				log.Printf("Error scraping %s: %v", item.Link, err)
				// 続行して他の記事を試す
				continue
			}

			summary, err = s.Summarize(ctx, content)
			if err != nil {
				log.Printf("Summarize error: %v", err)
				if summary == "" {
					summary = fallbackSummary(content)
				}
				// failure is non-fatal — we still save the article with fallback
			}

			tags, err = s.GenerateTags(ctx, content)
			if err != nil {
				log.Printf("GenerateTags error: %v", err)
				if tags == nil {
					tags = fallbackTags(content)
				}
			}
		}
		article := &model.Article{
			Title:       item.Title,
			URL:         item.Link,
			SourceName:  feed.Title,
			PublishedAt: pubDate,
			Content:     content,
			Summary:     &summary,
			Tags:        tags,
		}

		// ログを出して、1件のUpsertエラーで処理を中断しない
		fmt.Printf("Upserting article: %s (%s)\n", article.Title, article.URL)
		if err := s.repo.UpsertArticle(ctx, article); err != nil {
			fmt.Printf("Upsert error for %s: %v\n", article.URL, err)
			// 続行して他の記事を試す
			continue
		}
	}
	return nil
}

func (s *articleService) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
	return s.repo.ListArticles(ctx, limit)
}

// contentをスクレイピング
func (s *articleService) scrapeZennContent(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}

	selection := doc.Find(".znc")

	selection.Find(".TopicList_item___M3DS").Remove() // トピックタグ
	selection.Find(".embed-block").Remove()           // 埋め込みカード
	selection.Find("img").Remove()                    // 画像本体

	content := selection.Text()

	return strings.TrimSpace(content), nil
}

func (s *articleService) Summarize(ctx context.Context, content string) (string, error) {
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

	// プロンプトの組み立て
	prompt := genai.Text(fmt.Sprintf(
		"以下の技術記事の内容を、エンジニアが30秒で理解できるように3つの箇条書きで要約してください。\n"+
			"前置き、あいさつ、補足説明は不要です。要約本文だけをそのまま出力してください。\n\n"+
			"記事本文:\n%s",
		content,
	))

	resp, err := model.GenerateContent(ctx, prompt)
	if err != nil {
		if isQuotaError(err) {
			log.Printf("Gemini quota exceeded, using fallback summary: %v", err)
			return fallbackSummary(content), nil
		}
		return fallbackSummary(content), err
	}

	// レスポンスからテキストを抽出
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return normalizeSummary(fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])), nil
	}

	return fallbackSummary(content), nil
}

func (s *articleService) GenerateTags(ctx context.Context, content string) (*json.RawMessage, error) {
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
			log.Printf("Gemini quota exceeded, using fallback tags: %v", err)
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

func isQuotaError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "quota exceeded") ||
		strings.Contains(message, "429") ||
		regexp.MustCompile(`limit:\s*0`).MatchString(message)
}
