# RSS Tech Feed Summarizer

Zenn の RSS を収集し、Gemini で要約・タグ生成・埋め込み生成を行って Supabase に保存するアプリです。  
フロントエンドは React + Vite、バックエンドは Go + Echo で構成されています。

## 構成

- `backend`: API サーバー（Echo）
- `frontend`: Web UI（React + TypeScript + Vite）

## 前提

- Go `1.26.1`
- Node.js `18+`（推奨: `20+`）
- npm
- Supabase プロジェクト
- Gemini API キー

## セットアップ

### 1. バックエンド環境変数

`backend/.env` を作成し、以下を設定してください。

```env
SUPABASE_URL=https://<your-project>.supabase.co
SUPABASE_SERVICE_ROLE_KEY=<your_service_role_key>
# SERVICE_ROLE_KEY を使わない場合は ANON_KEY を設定
# SUPABASE_ANON_KEY=<your_anon_key>

GEMINI_API_KEY=<your_gemini_api_key>

# 任意（未設定時はデフォルト値を使用）
GEMINI_MODEL=gemini-2.5-flash-lite
EMBEDDING_MODEL=models/gemini-embedding-001
EMBEDDING_DIM=768
```

### 2. バックエンド起動

必ず `backend` ディレクトリで実行してください。

```bash
cd backend
go mod download
go run ./cmd/server
```

サーバーは `http://localhost:8080` で起動します。

### 3. フロントエンド起動

```bash
cd frontend
npm install
npm run dev
```

`http://localhost:5173` を開いてください。

## API

- `POST /fetch`  
  RSS を取得して要約・タグ・埋め込みを生成し、保存後に最新記事一覧を返します。
- `GET /articles`  
  最新記事一覧を返します。
- `GET /articles/recommended?id=<article_id>&limit=<n>`  
  指定記事に近い推薦記事を返します。

## Supabase テーブル例

最低限、`articles` テーブルに次の列が必要です。

- `id` (bigint, primary key)
- `title` (text)
- `url` (text, unique)
- `source_name` (text)
- `summary` (text, nullable)
- `published_at` (timestamptz)
- `created_at` (timestamptz)
- `content` (text)
- `tags` (jsonb, nullable)
- `embedding` (vector, nullable)

`embedding` 列は pgvector 拡張の導入が必要です。

## テスト

```bash
cd backend
go test ./... -v
```

## よくあるエラー

### `API error: 500`（フロントエンド表示）

1. バックエンドを `backend` ディレクトリで起動しているか確認する
2. `backend/.env` が存在し、`GEMINI_API_KEY` と Supabase の値が正しいか確認する
3. サーバーログに表示されたエラーを確認する（Gemini quota / Supabase 接続失敗など）

### `.envの読み込みに失敗しました`

- 実行ディレクトリが `backend` でない可能性があります。`cd backend` 後に起動してください。

## CI

GitHub Actions は `.github/workflows/ci.yml` で定義しています。

- backend: build + test
- frontend: lint + build
