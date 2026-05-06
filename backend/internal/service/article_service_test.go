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
type mockRepo struct {
	// existing を返すようにして、FetchOneUrl の挙動を切り替えられる
	existing *model.Article
	// saved に最後に Upsert された記事を保持する
	saved *model.Article
}

func (m *mockRepo) UpsertArticle(ctx context.Context, a *model.Article) error {
	// 保存された記事をコピーして保持
	copy := *a
	m.saved = &copy
	return nil
}

func (m *mockRepo) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
	return nil, nil
}

func (m *mockRepo) GetArticleByURL(ctx context.Context, url string) (*model.Article, error) {
	// テスト用に existing を使う
	return m.existing, nil
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

func TestFetchOneUrl_SkipSummarize(t *testing.T) {
	// feed サーバは同じく1件の item を返す
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0"?><rss><channel><title>test</title><item><title>one</title><link>http://example/1</link><pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate></item></channel></rss>`)
	}))
	defer ts.Close()

	// 既に summary がある既存記事を返すモックリポジトリ
	existing := &model.Article{
		URL:     "http://example/1",
		Content: "existing content",
	}
	s := "既存の要約"
	existing.Summary = &s

	repo := &mockRepo{existing: existing}
	svc := NewArticleService(repo)

	err := svc.(*articleService).FetchOneUrl(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	if repo.saved == nil {
		t.Fatalf("UpsertArticle was not called")
	}

	if repo.saved.Summary == nil || *repo.saved.Summary != "既存の要約" {
		t.Fatalf("Expected saved summary to be existing summary, got: %#v", repo.saved.Summary)
	}
}
