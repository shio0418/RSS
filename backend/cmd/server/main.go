// cmd/api/main.go (例)
package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/shio0418/RSS/internal/handler"
	"github.com/shio0418/RSS/internal/repository"
	"github.com/shio0418/RSS/internal/service"
	"github.com/supabase-community/supabase-go"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(".envの読み込みに失敗しました")
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	// サーバー側は service role キーを優先して使う。なければ anon をフォールバック。
	supabaseKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if supabaseKey == "" {
		supabaseKey = os.Getenv("SUPABASE_ANON_KEY")
	}

	supabaseClient, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		log.Fatalf("Supabaseの初期化に失敗しました: %v", err)
	}
	repo := repository.NewArticleRepository(supabaseClient)

	svc := service.NewArticleService(repo)

	hdl := handler.NewArticleHandler(svc)

	e := echo.New()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:5174"}, // Reactのポート
		AllowMethods: []string{echo.GET, echo.POST},
	}))

	e.POST("/fetch", hdl.FetchArticles)
	e.GET("/articles", hdl.ListArticles)
	e.GET("/articles/recommended", hdl.GetRecommendations)

	e.Logger.Fatal(e.Start(":8080"))
}
