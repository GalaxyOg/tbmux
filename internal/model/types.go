package model

import "time"

type RunRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	SourcePath    string    `json:"source_path"`
	WatchRoot     string    `json:"watch_root"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
	IsRunning     bool      `json:"is_running"`
}

type SelectionEntry struct {
	Source     string    `json:"source"`
	SelectedAt time.Time `json:"selected_at"`
}

type State struct {
	Version    int                       `json:"version"`
	UpdatedAt  time.Time                 `json:"updated_at"`
	Discovered []RunRecord               `json:"discovered"`
	Selected   map[string]SelectionEntry `json:"selected"`
}

type Filter struct {
	Today       bool
	Hours       int
	Days        int
	RunningOnly *bool
	Under       string
	Match       string
}

type DoctorItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type DoctorReport struct {
	CheckedAt time.Time    `json:"checked_at"`
	Items     []DoctorItem `json:"items"`
}
