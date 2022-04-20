package main

import (
	"fmt"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

func BytesBar(size int64, name string) *mpb.Bar {

	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}

	return p.AddBar(size,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
			decor.CountersKibiByte("[% .2f / % .2f]"),
		),
		mpb.AppendDecorators(
			decor.AverageETA(decor.ET_STYLE_GO),
			decor.Name(" | "),
			decor.AverageSpeed(decor.UnitKiB, "% .2f"),
		),
	)
}
