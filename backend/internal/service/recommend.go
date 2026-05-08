package service

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/shio0418/RSS/internal/model"
)

// コサイン類似度を計算
func cosineSimilarity(a, b []float64) float64 {
    if len(a) != len(b) || len(a) == 0 {
        return 0
    }

    var dotProduct float64
    var magnitudeA float64
    var magnitudeB float64

    for i := range a {
        dotProduct += a[i] * b[i]
        magnitudeA += a[i] * a[i]
        magnitudeB += b[i] * b[i]
    }

    magnitudeA = math.Sqrt(magnitudeA)
    magnitudeB = math.Sqrt(magnitudeB)

    if magnitudeA == 0 || magnitudeB == 0 {
        return 0
    }

    return dotProduct / (magnitudeA * magnitudeB)
}

// GetRecommendations は、指定された記事に基づいて推薦記事を取得
func (s *articleService) GetRecommendations(ctx context.Context, articleID int64, limit int) ([]model.Article, error) {
    articles, err := s.repo.ListArticles(ctx, 1000)
    if err != nil {
        return nil, err
    }

    var targetArticle *model.Article
    var otherArticles []model.Article

    for i, a := range articles {
        if a.ID == articleID {
            targetArticle = &articles[i]
        } else {
            otherArticles = append(otherArticles, a)
        }
    }

    if targetArticle == nil {
        return nil, fmt.Errorf("article not found: %d", articleID)
    }

    if len(targetArticle.Embedding) == 0 {
        return []model.Article{}, nil
    }

    type similarity struct {
        article model.Article
        score   float64
    }

    var similarities []similarity

    for _, article := range otherArticles {
        if len(article.Embedding) == 0 {
            continue
        }

        score := cosineSimilarity(targetArticle.Embedding, article.Embedding)
        similarities = append(similarities, similarity{article, score})
    }

    sort.Slice(similarities, func(i, j int) bool {
        return similarities[i].score > similarities[j].score
    })

    result := make([]model.Article, 0, limit)
    for i, sim := range similarities {
        if i >= limit {
            break
        }
        result = append(result, sim.article)
    }

    return result, nil
}
