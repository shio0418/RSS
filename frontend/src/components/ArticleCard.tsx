type ArticleCardProps = {
  title: string
  summary: string
  tags: string[]
  publishedLabel: string
  sourceName?: string
  imageEmoji?: string
  onLike?: () => void
  onDislike?: () => void
}

const ArticleCard = ({
  title,
  summary,
  tags,
  publishedLabel,
  sourceName,
  imageEmoji = '😊',
  onLike,
  onDislike,
}: ArticleCardProps) => {
  return (
    <article className="group w-full h-fit bg-white rounded-[24px] border border-gray-100 shadow-sm hover:shadow-xl transition-all duration-300 overflow-hidden flex flex-col p-6 sm:p-8 gap-6">
      <div
        className="w-full h-48 bg-gray-50 rounded-2xl flex items-center justify-center text-6xl group-hover:scale-105 transition-transform duration-300"
        aria-hidden="true"
      >
        {imageEmoji}
      </div>

      <div className="flex flex-col gap-3">
        <h2 className="text-xl font-bold text-blue-600 leading-tight line-clamp-2">{title}</h2>
        {sourceName ? <p className="text-xs text-gray-400">{sourceName}</p> : null}
        <p className="text-sm text-gray-500 leading-relaxed line-clamp-3">{summary}</p>
      </div>

      <div className="flex flex-wrap gap-2">
        {tags.map((tag, index) => (
          <span
            key={`${tag}-${index}`}
            className="px-3 py-1 bg-gray-100 text-gray-600 text-xs font-medium rounded-full hover:bg-blue-50 hover:text-blue-500 transition-colors"
          >
            {tag}
          </span>
        ))}
      </div>

      <footer className="flex justify-between items-end mt-auto pt-2 border-t border-gray-50">
        <div className="flex gap-3">
          <button
            type="button"
            className="p-2 rounded-full bg-pink-50 text-pink-500 hover:bg-pink-100 transition-colors"
            title="興味あり"
            onClick={onLike}
          >
            👍
          </button>
          <button
            type="button"
            className="p-2 rounded-full bg-cyan-50 text-cyan-500 hover:bg-cyan-100 transition-colors"
            title="興味なし"
            onClick={onDislike}
          >
            👎
          </button>
        </div>

        <div className="flex flex-col items-end">
          <span className="text-xs text-gray-400 font-mono">{publishedLabel}</span>
        </div>
      </footer>
    </article>
  )
}

export default ArticleCard