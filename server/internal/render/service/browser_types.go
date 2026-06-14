package service

import renderbrowser "github.com/RayleaBot/RayleaBot/server/internal/render/browser"

type Document = renderbrowser.Document
type Runner = renderbrowser.Runner
type ChromiumOptions = renderbrowser.ChromiumOptions

type closeableRunner interface {
	Close() error
}

func NewChromiumRunner(options ChromiumOptions) Runner {
	return renderbrowser.NewChromiumRunner(options)
}
