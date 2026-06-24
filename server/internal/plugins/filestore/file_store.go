package filestore

import (
	"path/filepath"
)

type Service struct {
	root string
}

func NewService(root string) *Service {
	return &Service{root: filepath.Clean(root)}
}
