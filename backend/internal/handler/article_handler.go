package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/shio0418/RSS/internal/service"
)

type ArticleHandler struct {
	svc service.ArticleService
}

func NewArticleHandler(svc service.ArticleService) *ArticleHandler {
	return &ArticleHandler{svc: svc}
}

func (h *ArticleHandler) FetchArticles(c echo.Context) error {
	urls := []string{
		"https://zenn.dev/feed",
	}

	ctx := c.Request().Context()
	err := h.svc.FetchAndSummarize(ctx, urls)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	// Fetchした直後に最新記事一覧を返す
	articles, err := h.svc.ListArticles(ctx, 100)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	return c.JSON(200, articles)
}

// GET /articles
func (h *ArticleHandler) ListArticles(c echo.Context) error {
    ctx := c.Request().Context()
    
    // 最新の20件くらいを取得してみる
    articles, err := h.svc.ListArticles(ctx, 20) 
    if err != nil {
        return c.JSON(500, map[string]string{"error": err.Error()})
    }
    
    return c.JSON(200, articles)
}

// GET /articles/recommended?id=<article_id>&limit=<limit>
func (h *ArticleHandler) GetRecommendations(c echo.Context) error {
	ctx := c.Request().Context()

	// クエリパラメータから article ID を取得
	articleIDStr := c.QueryParam("id")
	if articleIDStr == "" {
		return c.JSON(400, map[string]string{"error": "missing 'id' query parameter"})
	}

	articleID, err := strconv.ParseInt(articleIDStr, 10, 64)
	if err != nil {
		return c.JSON(400, map[string]string{"error": "invalid 'id' parameter"})
	}

	// limit パラメータを取得（デフォルト: 10）
	limitStr := c.QueryParam("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 推薦記事を取得
	recommendations, err := h.svc.GetRecommendations(ctx, articleID, limit)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	return c.JSON(200, recommendations)
}