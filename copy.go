package main

import (
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"os"
)

func initCopy(opt *options) {
	root, err := NewFile(opt.Copy.Path)
	if err != nil {
		log.Fatal(err)
	}
	fs, err := ListFilesLocal(root)
	if err != nil {
		log.Fatal(err)
	}

	for i, f := range fs {
		target := f.GetTarget(opt.Copy.Path, opt.Copy.Remote)
		target.IsLocal = true
		if err := target.CheckParent(); err != nil {
			log.Fatal(err)
		}

		if stat, err := os.Stat(target.Path); !os.IsNotExist(err) {
			target.Size = stat.Size()
		}

		if target.Size == f.Size {
			log.Infof("skip: %s", f.Path)
			continue
		} else if target.Size > f.Size {
			log.Warnf("%s is corrupted", target.Path)
			err = os.Remove(target.Path)
			if err != nil {
				log.Warnf("failed to remove %s: %v", target.Path, err)
				continue
			}
			target.Size = 0
		}

		r, err := os.Open(f.Path)
		if err != nil {
			log.Warnf("failed to open %s: %v", f.Path, err)
			continue
		}

		w, err := os.OpenFile(target.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			log.Warnf("failed to open %s: %v", target.Path, err)
			_ = r.Close()
			continue
		}

		bar := BytesBar(f.Size-target.Size, fmt.Sprintf("[%d/%d] %s", i+1, len(fs), f.Name()))
		if _, err := r.Seek(target.Size, 0); err != nil {
			log.Warnf("failed to seek %s: %v", f.Path, err)
			_ = r.Close()
			_ = w.Close()
			continue
		}

		// create proxy reader
		reader := progressbar.NewReader(r, bar)

		_, err = io.Copy(w, &reader)
		_ = bar.Finish()
		_ = reader.Close()
		_ = w.Close()
		_ = r.Close()
	}
}
