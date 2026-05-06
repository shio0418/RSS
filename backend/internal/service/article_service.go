package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
	"github.com/shio0418/RSS/internal/model"
	"github.com/shio0418/RSS/internal/repository"
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
		content, _ := s.scrapeZennContent(item.Link)
		article := &model.Article{
			Title:       item.Title,
			URL:         item.Link,
			SourceName:  feed.Title,
			PublishedAt: pubDate,
			Content: content,
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

	// 1. まずは本文の器（znc）を特定する
	selection := doc.Find(".znc")

	// 2. 不要な「画像説明」や「埋め込みカード」を削除してノイズを減らす
	selection.Find(".TopicList_item___M3DS").Remove() // トピックタグ
	selection.Find(".embed-block").Remove()           // 埋め込みカード
	selection.Find("img").Remove()                    // 画像本体

	// 3. テキストを抽出
	// Zennの場合、コードブロック (<pre>) も .znc の中にあるので、
	// .Text() を使うとコードの中身も文字列として取得できます。
	content := selection.Text()

	return strings.TrimSpace(content), nil
}
