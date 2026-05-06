import { useEffect, useState } from 'react'

type Article = {
  url: string
  title: string
  source_name: string
  summary: string | null
}

function App() {
  const [articles, setArticles] = useState<Article[]>([])
  const [loading, setLoading] = useState(false)

  const loadArticles = async () => {
    const res = await fetch('http://localhost:8080/articles')
    const data = await res.json()
    setArticles(data)
  }

  const fetchAndRefresh = async () => {
    setLoading(true)

    try {
      await fetch('http://localhost:8080/fetch', { method: 'POST' })
      await loadArticles()
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetch('http://localhost:8080/articles')
      .then(res => res.json())
      .then(data => setArticles(data))
      .catch(err => console.error(err))
  }, [])

  return (
    <div style={{ padding: '20px' }}>
      <h1>RSS Reader</h1>
      <button type="button" onClick={() => void fetchAndRefresh()} disabled={loading}>
        {loading ? '更新中...' : '記事を取得して要約'}
      </button>
      <ul>
        {articles.map((article, index) => (
          <li key={index}>
            <a href={article.url} target="_blank" rel="noreferrer">
              {article.title}
            </a>
            <span style={{ marginLeft: '10px', color: '#666', fontSize: '0.8em' }}>
              ({article.source_name})
            </span>
            <div style={{ marginTop: '6px', color: '#333' }}>
              {article.summary ?? '要約なし'}
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

export default App