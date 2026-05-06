package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shio0418/RSS/internal/model"
)

// テスト用の偽物リポジトリ
type mockRepo struct{}

func (m *mockRepo) UpsertArticle(ctx context.Context, a *model.Article) error {
	// ここで print すれば、実際にデータが流れてきたか目視確認できる
	println("Saving to DB:", a.Title)
	return nil
}

func (m *mockRepo) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
	return nil, nil
}

func (m *mockRepo) GetArticleByURL(ctx context.Context, url string) (*model.Article, error) {
	return nil, nil
}

func TestFetchOneUrl(t *testing.T) {
	// httptest で簡易 feed を返すサーバを立てる
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0"?><rss><channel><title>test</title><item><title>one</title><link>http://example/1</link><pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate></item></channel></rss>`)
	}))
	defer ts.Close()

	repo := &mockRepo{}
	svc := NewArticleService(repo)

	err := svc.(*articleService).FetchOneUrl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
}
