package repository

import (
	"context"

	"github.com/shio0418/RSS/internal/model"
	"github.com/supabase-community/postgrest-go"
	"github.com/supabase-community/supabase-go"
)

// ArticleRepository はDB操作のインターフェース
type ArticleRepository interface {
	UpsertArticle(ctx context.Context, article *model.Article) error
	ListArticles(ctx context.Context, limit int) ([]model.Article, error)
	GetArticleByURL(ctx context.Context, url string) (*model.Article, error)
}

// supabaseRepository はインターフェースの実体
type supabaseRepository struct {
	client *supabase.Client
}

// NewArticleRepository はレポジトリのコンストラクタ
func NewArticleRepository(client *supabase.Client) ArticleRepository {
	return &supabaseRepository{
		client: client,
	}
}

// UpsertArticle は記事を保存、すでにあれば更新
func (r *supabaseRepository) UpsertArticle(ctx context.Context, a *model.Article) error {
	// Upsert(json, onConflict, resolution, count)
	// countは通常空文字 "" でOKです
	_, _, err := r.client.From("articles").Upsert(a, "url", "exact", "").Execute()
	return err
}

// ListArticles は最新の記事を一覧取得
func (r *supabaseRepository) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
	var articles []model.Article

	// Select(columns, head, count)
	// Order(column, ascending, nullsFirst, foreignTable)
	// Limit(count, foreignTable)
	_, err := r.client.From("articles").
		Select("*", "exact", false).
		Order("published_at", &postgrest.OrderOpts{
			Ascending:  false,
			NullsFirst: false,
		}).
		Limit(limit, "").
		ExecuteTo(&articles)

	return articles, err
}

// GetArticleByURL は URL に紐づく記事を1件取得します。見つからなければ (nil, nil) を返します。
func (r *supabaseRepository) GetArticleByURL(ctx context.Context, url string) (*model.Article, error) {
	var articles []model.Article

	_, err := r.client.From("articles").
		Select("*", "exact", false).
		Eq("url", url).
		Limit(1, "").
		ExecuteTo(&articles)

	if err != nil {
		return nil, err
	}
	if len(articles) == 0 {
		return nil, nil
	}
	return &articles[0], nil
}
