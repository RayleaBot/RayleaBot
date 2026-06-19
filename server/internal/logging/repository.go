package logging

import (
	"context"
	"errors"
	"time"
)

type Query struct {
	Level     string
	Levels    []string
	Source    string
	Protocol  string
	PluginID  string
	PluginIDs []string
	RequestID string
	BootID    string
	StartAt   string
	EndAt     string
	Limit     int
}

type PageDirection string

const (
	PageDirectionOlder PageDirection = "older"
	PageDirectionNewer PageDirection = "newer"
)

type PageQuery struct {
	Level     string
	Levels    []string
	Source    string
	Protocol  string
	PluginID  string
	PluginIDs []string
	RequestID string
	BootID    string
	StartAt   string
	EndAt     string
	Limit     int
	Cursor    string
	Direction PageDirection
}

type PageInfo struct {
	Limit       int     `json:"limit"`
	HasOlder    bool    `json:"has_older"`
	HasNewer    bool    `json:"has_newer"`
	OlderCursor *string `json:"older_cursor"`
	NewerCursor *string `json:"newer_cursor"`
}

type PageResult struct {
	Items []Summary `json:"items"`
	Page  PageInfo  `json:"page"`
}

var ErrLogNotFound = errors.New("management log not found")
var ErrInvalidCursor = errors.New("management log cursor is invalid")

type Repository interface {
	SaveSummary(context.Context, Summary) error
	ListSummaries(context.Context, Query) ([]Summary, error)
	ListPage(context.Context, PageQuery) (PageResult, error)
	GetSummary(context.Context, string) (Summary, error)
	PruneOlderThan(context.Context, time.Time) error
}
