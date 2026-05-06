package service

import (
	"context"
	"github.com/shio0418/RSS/internal/model"
	"github.com/shio0418/RSS/internal/repository"
	"github.com/mmcdole/gofeed"
	"sync"
)

type ArticleService interface {
    FetchAndSummarize(ctx context.Context, urls []string) error
}

type articleService struct {
	repo repository.ArticleRepository
}

// コンストラクタ
func NewArticleService(repo repository.ArticleRepository) ArticleService {
	return &articleService {
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
				_ = s.FetchOneUrl(ctx, url)
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
		article := &model.Article {
			Title: item.Title,
			URL: item.Link,
			SourceName: feed.Title,
			PublishedAt: *item.PublishedParsed,
		}

		err := s.repo.UpsertArticle(ctx, article)
		if err != nil {
			return err
		}
	}
	return nil
}