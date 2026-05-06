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
		"https://news.yahoo.co.jp/rss/topics/top-picks.xml",
	}

	ctx := c.Request().Context()
	err := h.svc.FetchAndSummarize(ctx, urls)
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}

	return c.JSON(200, map[string]string{"message": "success"})
}
