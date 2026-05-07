import { useEffect, useState } from 'react'
import ArticleCard from './components/ArticleCard'

type Article = {
  id?: number
  title: string
  url: string
  source_name: string
  summary: string | null
  published_at: string
  tags?: string | string[] | null
}

function getRelativeTime(isoDate: string): string {
  const now = new Date()
  const date = new Date(isoDate)
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return '今'
  if (diffMins < 60) return `${diffMins}分前`
  if (diffHours < 24) return `${diffHours}時間前`
  if (diffDays < 7) return `${diffDays}日前`
  return date.toLocaleDateString('ja-JP')
}

function parseTags(tags: string | string[] | null | undefined): string[] {
  if (!tags) return []
  // Already an array from backend
  if (Array.isArray(tags)) return tags
  // Try to parse as JSON string
  try {
    const parsed = JSON.parse(tags)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function App() {
  const [articles, setArticles] = useState<Article[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchArticles = async () => {
      try {
        setLoading(true)
        setError(null)
        const res = await fetch('http://localhost:8080/articles')
        if (!res.ok) throw new Error(`API error: ${res.status}`)
        const data: Article[] = await res.json()
        setArticles(data || [])
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch articles')
        console.error(err)
      } finally {
        setLoading(false)
      }
    }

    fetchArticles()
  }, [])

  return (
    <main className="min-h-screen bg-slate-50 py-10 px-4 sm:px-8">
      <div className="max-w-6xl mx-auto">
        <header className="mb-8">
          <h1 className="text-3xl sm:text-4xl font-bold text-slate-800">RSS Reader</h1>
          <p className="text-slate-500 mt-2">最新の記事</p>
        </header>

        {loading && <p className="text-slate-600">読み込み中...</p>}
        {error && <p className="text-red-600">エラー: {error}</p>}

        {!loading && articles.length === 0 && <p className="text-slate-600">記事がありません</p>}

        {!loading && articles.length > 0 && (
          <section className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
            {articles.map((article) => {
              const parsedTags = parseTags(article.tags)
              if (article.tags && parsedTags.length === 0) {
                console.log('Tags parse issue - raw:', article.tags, 'parsed:', parsedTags)
              }
              return (
                <ArticleCard
                  key={article.url}
                  title={article.title}
                  summary={article.summary || '要約がありません'}
                  tags={parsedTags}
                  sourceName={article.source_name}
                  publishedLabel={getRelativeTime(article.published_at)}
                  onLike={() => console.log('like:', article.id)}
                  onDislike={() => console.log('dislike:', article.id)}
                />
              )
            })}
          </section>
        )}
      </div>
    </main>
  )
}

export default App