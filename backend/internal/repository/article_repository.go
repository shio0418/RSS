package repository

import (
	"context"
	"github.com/shio0418/RSS/internal/model"
	"github.com/supabase-community/supabase-go"
)

// ArticleRepository はDB操作のインターフェース
type ArticleRepository interface {
	UpsertArticle(ctx context.Context, article *model.Article) error
	ListArticles(ctx context.Context, limit int) ([]model.Article, error)
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

// UpsertArticle は記事を保存、すでにあれば更新（重複防止）
func (r *supabaseRepository) UpsertArticle(ctx context.Context, a *model.Article) error {
	// Supabaseのクライアントを使って、articlesテーブルにデータを投げる
	// "on_conflict" を使うことで、URLの重複を検知して更新(Upsert)にする
	_, _, err := r.client.From("articles").Upsert(a, "url", "exact").Execute()
	return err
}

// ListArticles は最新の記事を一覧取得
func (r *supabaseRepository) ListArticles(ctx context.Context, limit int) ([]model.Article, error) {
	var articles []model.Article
	err := r.client.From("articles").
		Select("*").
		Order("published_at", &supabase.OrderOptions{Ascending: false}).
		Limit(limit).
		ExecuteTo(&articles)
	
	return articles, err
}