package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shio0418/RSS/internal/model"
)

const testArticleHTML = `<html>
<body>
  <div class="znc">
    <p>hello</p>
    <div class="TopicList_item___M3DS">topic</div>
    <div class="embed-block">embed</div>
    <img src="/x.png"/>
    <p>world</p>
  </div>
</body>
</html>`

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
	t.Setenv("GEMINI_API_KEY", "")

	contentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, testArticleHTML)
	}))
	defer contentServer.Close()

	// httptest で簡易 feed を返すサーバを立てる
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		io.WriteString(w, `<?xml version="1.0"?><rss><channel><title>test</title><item><title>one</title><link>`+contentServer.URL+`</link><pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate></item></channel></rss>`)
	}))
	defer feedServer.Close()

	repo := &mockRepo{}
	svc := NewArticleService(repo)

	err := svc.(*articleService).FetchOneUrl(context.Background(), feedServer.URL)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	if repo.saved == nil {
		t.Fatalf("UpsertArticle was not called")
	}

	if repo.saved.Title != "one" {
		t.Fatalf("expected saved title %q, got %q", "one", repo.saved.Title)
	}

	if repo.saved.URL != contentServer.URL {
		t.Fatalf("expected saved URL %q, got %q", contentServer.URL, repo.saved.URL)
	}

	if repo.saved.SourceName != "test" {
		t.Fatalf("expected saved source %q, got %q", "test", repo.saved.SourceName)
	}

	if !strings.Contains(repo.saved.Content, "hello") {
		t.Fatalf("expected content to contain %q, got %q", "hello", repo.saved.Content)
	}

	if !strings.Contains(repo.saved.Content, "world") {
		t.Fatalf("expected content to contain %q, got %q", "world", repo.saved.Content)
	}

	if strings.Contains(repo.saved.Content, "topic") {
		t.Fatalf("expected content to not contain %q (filtered element), got %q", "topic", repo.saved.Content)
	}

	if strings.Contains(repo.saved.Content, "embed") {
		t.Fatalf("expected content to not contain %q (filtered element), got %q", "embed", repo.saved.Content)
	}

	if repo.saved.Summary == nil || *repo.saved.Summary == "" {
		t.Fatalf("expected non-empty summary to be saved")
	}

	if !strings.Contains(*repo.saved.Summary, "hello") {
		t.Fatalf("expected summary to include scraped content, got %q", *repo.saved.Summary)
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

func TestNormalizeSummary(t *testing.T) {
	input := "わかりました。\n- 1つ目のポイント\n- 2つ目のポイント\n- 3つ目のポイント"

	got := normalizeSummary(input)
	if strings.Contains(got, "わかりました") {
		t.Fatalf("expected acknowledgement to be removed, got: %q", got)
	}

	expected := "- 1つ目のポイント\n- 2つ目のポイント\n- 3つ目のポイント"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
