package memory

import (
	"encoding/json"
	"time"
)

type Entry struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Sequence  int64     `json:"sequence"`
}

func (e *Entry) UnmarshalJSON(data []byte) error {
	type entryAlias struct {
		ID              string    `json:"id"`
		Source          string    `json:"source"`
		Content         string    `json:"content"`
		CreatedAt       time.Time `json:"created_at"`
		LegacyCreatedAt time.Time `json:"createdAt"`
		Sequence        int64     `json:"sequence"`
	}

	var aux entryAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	e.ID = aux.ID
	e.Source = aux.Source
	e.Content = aux.Content
	e.Sequence = aux.Sequence
	if !aux.CreatedAt.IsZero() {
		e.CreatedAt = aux.CreatedAt
	} else {
		e.CreatedAt = aux.LegacyCreatedAt
	}

	return nil
}
