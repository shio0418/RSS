package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
	"github.com/shio0418/RSS/internal/model"
)

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

		var content string
		var summary string
		var tags *json.RawMessage

		existing, err := s.repo.GetArticleByURL(ctx, item.Link)
		if err != nil {
			log.Printf("GetArticleByURL error: %v", err)
		}

		if existing != nil && existing.Summary != nil && *existing.Summary != "" {
			summary = *existing.Summary
			content = existing.Content
			tags = existing.Tags
			// もし以前の要約がフォールバック（本文冒頭表示）だったら再度要約を試みる
			if isFallbackSummary(summary) {
				var sourceText string
				if strings.TrimSpace(content) != "" {
					sourceText = content
				} else {
					// フォールバック先の本文抜粋が含まれている場合はプレフィックスを除去して試す
					sourceText = strings.TrimPrefix(summary, "要約を生成できなかったため、本文の冒頭を表示します:")
				}

				newSummary, err := s.Summarize(ctx, sourceText)
				if err == nil && strings.TrimSpace(newSummary) != "" && !isFallbackSummary(newSummary) {
					summary = newSummary
				}
			}

			if !hasNonEmptyTags(tags) {
				tagSource := content
				if strings.TrimSpace(tagSource) == "" {
					tagSource = summary
				}

				tags, err = s.GenerateTags(ctx, tagSource)
				if err != nil {
					log.Printf("GenerateTags error: %v", err)
					if !hasNonEmptyTags(tags) {
						tags = fallbackTags(tagSource)
					}
				}
			}
		} else {
			content, err = s.scrapeZennContent(item.Link)
			if err != nil {
				log.Printf("Error scraping %s: %v", item.Link, err)
				continue
			}

			summary, err = s.Summarize(ctx, content)
			if err != nil {
				log.Printf("Summarize error: %v", err)
				if summary == "" {
					summary = fallbackSummary(content)
				}
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

		embeddingText := fmt.Sprintf("Title: %s\nSummary: %s", item.Title, summary)
		embedding, err := s.GenerateEmbedding(ctx, embeddingText)
		if err != nil {
			log.Printf("GenerateEmbedding error: %v", err)
		} else if embedding != nil {
			article.Embedding = embedding
		}

		fmt.Printf("Upserting article: %s (%s)\n", article.Title, article.URL)
		if err := s.repo.UpsertArticle(ctx, article); err != nil {
			fmt.Printf("Upsert error for %s: %v\n", article.URL, err)
			continue
		}
	}
	return nil
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

	selection.Find(".TopicList_item___M3DS").Remove()
	selection.Find(".embed-block").Remove()
	selection.Find("img").Remove()

	content := selection.Text()

	return strings.TrimSpace(content), nil
}
