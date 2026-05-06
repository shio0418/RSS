package service

import (
	"context"
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
		content, err := s.scrapeZennContent(item.Link)
		if err != nil {
			return err
		}

		summary, err := s.Summarize(ctx, content)
		if err != nil {
			log.Printf("Summarize error: %v", err)
			return err
		}
		article := &model.Article{
			Title:       item.Title,
			URL:         item.Link,
			SourceName:  feed.Title,
			PublishedAt: pubDate,
			Content:     content,
			Summary:     &summary,
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
		"以下の技術記事の内容を、エンジニアが30秒で理解できるように3つの箇条書きで要約してください。\n\n記事本文:\n%s",
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
		return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
	}

	return fallbackSummary(content), nil
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

func isQuotaError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "quota exceeded") ||
		strings.Contains(message, "429") ||
		regexp.MustCompile(`limit:\s*0`).MatchString(message)
}
