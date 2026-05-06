package model

import "time"

type Article struct {
	ID          int64     `json:"id,omitempty" db:"id"`
	Title       string    `json:"title" db:"title"`
	URL         string    `json:"url" db:"url"`
	SourceName  string    `json:"source_name" db:"source_name"`
	Summary     *string   `json:"summary" db:"summary"` // Nullableなのでポインタ
	PublishedAt time.Time `json:"published_at" db:"published_at"`
	CreatedAt   time.Time `json:"created_at,omitempty" db:"created_at"`
    Content    string    `json:"content" db:"content"`
}
