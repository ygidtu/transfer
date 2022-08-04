package main

import (
	"fmt"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
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

func BytesBar(size int64, name string) *mpb.Bar {

	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}

	return progress.New(
		size,
		mpb.BarStyle().Lbound("╢").Filler("▌").Tip("▌").Padding("░").Rbound("╟"),
		// override default "[=>-]" style
		mpb.PrependDecorators(
			// display our name with one space on the right
			decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
		),
		mpb.AppendDecorators(
			decor.AverageETA(decor.ET_STYLE_GO),
			decor.Name(" "),
			decor.AverageSpeed(decor.UnitKB, "% .2f"),
			decor.Name(" ["),
			decor.CountersKibiByte("% .2f / % .2f"),
			decor.Name(" ]"),
		),
	)
}
