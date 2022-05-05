package main

import (
	"fmt"
	"path/filepath"
)

// File is used to kept file path and size
type File struct {
	Path string
	Size int64
}

func (file *File) Name() string {
	return filepath.Base(file.Path)
}

// ByteCountDecimal human-readable file size
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
