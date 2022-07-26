package main

import (
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
		log.Infof("[%d/%d]%s", i+1, len(fs), f.Name())

		target := f.GetTarget(opt.Copy.Path, opt.Copy.Remote)
		if stat, err := os.Stat(target.Path); os.IsNotExist(err) {
			target.Size = stat.Size()
		}

		if target.Size == f.Size {
			log.Infof("skip: %s", f.Path)
			continue
		}

		r, err := os.Open(f.Path)
		if err != nil {
			log.Warnf("failed to open %s: %v", f.Path, err)
			continue
		}
		defer r.Close()

		w, err := os.OpenFile(target.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			log.Warnf("failed to open %s: %v", f.Path, err)
			continue
		}
		defer w.Close()

		if target.Size > f.Size {
			log.Warnf("%s is corrupted")
			err = os.Remove(target.Path)
			if err != nil {
				log.Warnf("failed to remove %s: %v", target.Path, err)
				continue
			}
			target.Size = 0
		}

		bar := BytesBar(f.Size-target.Size, f.Name())
		defer bar.Finish()

		if _, err := r.Seek(target.Size, 0); err != nil {
			log.Warnf("failed to seek %s: %v", f.Path, err)
			continue
		}

		// create proxy reader
		reader := progressbar.NewReader(r, bar)
		defer reader.Close()
		_, err = io.Copy(w, &reader)
		io.Copy(w, r)
	}
}
