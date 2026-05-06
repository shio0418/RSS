package handler

import (
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
    articles, err := h.svc.ListArticles(ctx, 20) // svcにもListが必要ですね
    if err != nil {
        return c.JSON(500, map[string]string{"error": err.Error()})
    }
    
    return c.JSON(200, articles)
}