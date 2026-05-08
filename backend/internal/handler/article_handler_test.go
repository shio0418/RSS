package handler

import (
    "context"
    "encoding/json"
    "io"
    "net/http/httptest"
    "testing"

    "github.com/labstack/echo/v4"
    "github.com/shio0418/RSS/internal/model"
)

// mockService implements the service interface used by the handler
type mockService struct{
    // track whether FetchAndSummarize was called and with what
    CalledFetch bool
    ReceivedUrls []string
    ArticlesToReturn []model.Article
}

func (m *mockService) FetchAndSummarize(ctx context.Context, urls []string) error {
    m.CalledFetch = true
    m.ReceivedUrls = urls
    return nil
}

func (m *mockService) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
    return m.ArticlesToReturn, nil
}

func (m *mockService) GetRecommendations(ctx context.Context, articleID int64, limit int) ([]model.Article, error) {
    return m.ArticlesToReturn, nil
}

func TestFetchArticlesHandler(t *testing.T) {
    e := echo.New()

    sample := model.Article{
        ID: 1,
        Title: "t",
        URL: "https://example/1",
        SourceName: "src",
    }

    svc := &mockService{ArticlesToReturn: []model.Article{sample}}
    h := NewArticleHandler(svc)

    req := httptest.NewRequest("POST", "/fetch", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    if err := h.FetchArticles(c); err != nil {
        t.Fatalf("FetchArticles returned error: %v", err)
    }

    if rec.Code != 200 {
        t.Fatalf("expected status 200, got %d", rec.Code)
    }

    // body should be JSON array with our sample article
    body, _ := io.ReadAll(rec.Body)
    var got []model.Article
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("failed to unmarshal response: %v; body: %s", err, string(body))
    }

    if len(got) != 1 || got[0].URL != sample.URL {
        t.Fatalf("unexpected response articles: %#v", got)
    }

    if !svc.CalledFetch {
        t.Fatalf("expected FetchAndSummarize to be called")
    }
}

func TestListArticlesHandler(t *testing.T) {
    e := echo.New()

    sample := model.Article{
        ID: 2,
        Title: "u",
        URL: "https://example/2",
        SourceName: "src2",
    }

    svc := &mockService{ArticlesToReturn: []model.Article{sample}}
    h := NewArticleHandler(svc)

    req := httptest.NewRequest("GET", "/articles", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)

    if err := h.ListArticles(c); err != nil {
        t.Fatalf("ListArticles returned error: %v", err)
    }

    if rec.Code != 200 {
        t.Fatalf("expected status 200, got %d", rec.Code)
    }

    body, _ := io.ReadAll(rec.Body)
    var got []model.Article
    if err := json.Unmarshal(body, &got); err != nil {
        t.Fatalf("failed to unmarshal response: %v; body: %s", err, string(body))
    }

    if len(got) != 1 || got[0].ID != sample.ID {
        t.Fatalf("unexpected response articles: %#v", got)
    }
}
