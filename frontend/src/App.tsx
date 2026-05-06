import { useEffect, useState } from 'react'

type Article = {
  url: string
  title: string
  source_name: string
}

function App() {
  const [articles, setArticles] = useState<Article[]>([])

  useEffect(() => {
    // バックエンドから記事を取得
    fetch('http://localhost:8080/articles')
      .then(res => res.json())
      .then(data => setArticles(data))
      .catch(err => console.error(err))
  }, [])

  return (
    <div style={{ padding: '20px' }}>
      <h1>RSS Reader</h1>
      <ul>
        {articles.map((article, index) => (
          <li key={index}>
            <a href={article.url} target="_blank" rel="noreferrer">
              {article.title}
            </a>
            <span style={{ marginLeft: '10px', color: '#666', fontSize: '0.8em' }}>
              ({article.source_name})
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}

export default App