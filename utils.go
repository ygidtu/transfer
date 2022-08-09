package main

import (
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"os"
	"time"
)

func Copy(reader io.Reader, writer io.Writer) error {
	var err error
	for {
		byteBuff := make([]byte, 1024*32)
		_, er := reader.Read(byteBuff)
		if er != nil {
			if er == io.EOF {
				break
			} else {
				err = er
				break
			}
		}

		_ = bar.Add(len(byteBuff))
		_, err = writer.Write(byteBuff)

		if err != nil {
			break
		}
	}
	return err
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

func BytesBar(size int64, name string) *progressbar.ProgressBar {

	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}

	return progressbar.NewOptions(int(size),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetDescription(name),
		progressbar.OptionFullWidth(),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSetWidth(10),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
}
