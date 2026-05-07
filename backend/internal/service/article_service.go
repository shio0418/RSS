package service

import (
	"context"

	"github.com/shio0418/RSS/internal/model"
	"github.com/shio0418/RSS/internal/repository"
)

type ArticleService interface {
    FetchAndSummarize(ctx context.Context, urls []string) error
    ListArticles(ctx context.Context, limit int) ([]model.Article, error)
    GetRecommendations(ctx context.Context, articleID int64, limit int) ([]model.Article, error)
}

type articleService struct {
    repo repository.ArticleRepository
}

// コンストラクタ
func NewArticleService(repo repository.ArticleRepository) ArticleService {
    return &articleService{repo: repo}
}

func (s *articleService) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
    return s.repo.ListArticles(ctx, limit)
}
