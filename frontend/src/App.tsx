import ArticleCard from './components/ArticleCard'

type MockArticle = {
  id: string
  title: string
  summary: string
  tags: string[]
  sourceName: string
  publishedLabel: string
  imageEmoji: string
}

const mockArticles: MockArticle[] = [
  {
    id: 'a1',
    title: 'ReactとGoで作る全文検索エンジンの裏側',
    summary: 'Gemini APIで要約した記事を保存し、検索までつなぐ実装フローを整理した記事です。',
    tags: ['Go', 'React', 'Gemini'],
    sourceName: 'Zenn',
    publishedLabel: '3時間前',
    imageEmoji: '🔎',
  },
  {
    id: 'a2',
    title: 'RAGパイプラインにおける失敗時フォールバック設計',
    summary: '外部APIが失敗した時でも読みやすい要約を返すためのフォールバック戦略を解説します。',
    tags: ['LLM', 'RAG', 'Backend'],
    sourceName: 'Qiita',
    publishedLabel: '昨日',
    imageEmoji: '🛟',
  },
  {
    id: 'a3',
    title: 'テストを安定化する: httptestで外部依存を消す',
    summary: 'CIで詰まりやすい外部HTTP依存を、httptestでローカル完結にする方法を紹介します。',
    tags: ['Go Test', 'CI', 'Quality'],
    sourceName: 'Tech Blog',
    publishedLabel: '2日前',
    imageEmoji: '🧪',
  },
]

function App() {
  return (
    <main className="min-h-screen bg-slate-50 py-10 px-4 sm:px-8">
      <div className="max-w-6xl mx-auto">
        <header className="mb-8">
          <h1 className="text-3xl sm:text-4xl font-bold text-slate-800">RSS Reader</h1>
          <p className="text-slate-500 mt-2">モック記事カード一覧</p>
        </header>

        <section className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {mockArticles.map((article) => (
            <ArticleCard
              key={article.id}
              title={article.title}
              summary={article.summary}
              tags={article.tags}
              sourceName={article.sourceName}
              publishedLabel={article.publishedLabel}
              imageEmoji={article.imageEmoji}
              onLike={() => console.log('like:', article.id)}
              onDislike={() => console.log('dislike:', article.id)}
            />
          ))}
        </section>
      </div>
    </main>
  )
}

export default App