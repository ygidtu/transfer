package main

import (
	"fmt"
	"github.com/schollz/progressbar/v3"
)

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

// BytesBar is used to generate progress bar
func BytesBar(size int64, name string) *progressbar.ProgressBar {

	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}

	return progressbar.DefaultBytes(size, name)
}
