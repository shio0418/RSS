package model

import (
	"encoding/json"
	"strings"
	"time"
)

type Article struct {
	ID          int64            `json:"id,omitempty" db:"id"`
	Title       string           `json:"title" db:"title"`
	URL         string           `json:"url" db:"url"`
	SourceName  string           `json:"source_name" db:"source_name"`
	Summary     *string          `json:"summary" db:"summary"` // Nullableなのでポインタ
	PublishedAt time.Time        `json:"published_at" db:"published_at"`
	CreatedAt   time.Time        `json:"created_at,omitempty" db:"created_at"`
	Content     string           `json:"content" db:"content"`
	Tags        *json.RawMessage `json:"tags" db:"tags"`
	Embedding   []float64        `json:"embedding,omitempty" db:"embedding"`
}

func (a *Article) UnmarshalJSON(data []byte) error {
	type articleAlias Article
	aux := struct {
		Embedding json.RawMessage `json:"embedding"`
		*articleAlias
	}{
		articleAlias: (*articleAlias)(a),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.Embedding) == 0 || string(aux.Embedding) == "null" {
		a.Embedding = nil
		return nil
	}

	var vec []float64
	if err := json.Unmarshal(aux.Embedding, &vec); err == nil {
		a.Embedding = vec
		return nil
	}

	var embeddingText string
	if err := json.Unmarshal(aux.Embedding, &embeddingText); err == nil {
		embeddingText = strings.TrimSpace(embeddingText)
		if embeddingText == "" {
			a.Embedding = nil
			return nil
		}

		if err := json.Unmarshal([]byte(embeddingText), &vec); err == nil {
			a.Embedding = vec
			return nil
		}
	}

	// embedding が不正形式でも、一覧取得を失敗させない
	a.Embedding = nil
	return nil
}
