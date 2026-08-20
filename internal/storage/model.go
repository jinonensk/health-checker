package storage

import "time"

// Site представляет сайт для мониторинга
type Site struct {
	ID           int        `json:"id"`
	URL          string     `json:"url"`
	IntervalSec  int        `json:"interval_sec"`
	LastCheck    *time.Time `json:"last_check,omitempty"`
	LastStatus   *int       `json:"last_status,omitempty"`   // HTTP код или nil
	ResponseTime *int       `json:"response_time,omitempty"` // мс
	CreatedAt    time.Time  `json:"created_at"`
}
