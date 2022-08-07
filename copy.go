package main

import (
	"fmt"
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

	if root, err := NewFile(opt.Copy.Path); err != nil {
		log.Fatal(err)
	} else {
		source = root
	}

	if root, err := NewFile(opt.Copy.Remote); err != nil {
		log.Fatal(err)
	} else {
		target = root
	}

	taskChan := make(chan *File)
	for i := 0; i < opt.Concurrent; i++ {
		go func() {
			defer wg.Done()
			for {
				f, ok := <-taskChan

				if !ok {
					break
				}

				target := f.GetTarget(source, target)
				target.IsLocal = true
				if err := target.CheckParent(); err != nil {
					log.Fatal(err)
				}

				if stat, err := os.Stat(target.Path); !os.IsNotExist(err) {
					target.Size = stat.Size()
				}

				if target.Size == f.Size {
					log.Infof("skip: %s", f.Path)
					return
				} else if target.Size > f.Size {
					log.Warnf("%s is corrupted", target.Path)
					err = os.Remove(target.Path)
					if err != nil {
						log.Warnf("failed to remove %s: %v", target.Path, err)
						return
					}
					target.Size = 0
				}

				r, err := os.Open(f.Path)
				if err != nil {
					log.Warnf("failed to open %s: %v", f.Path, err)
					return
				}

				w, err := os.OpenFile(target.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
				if err != nil {
					log.Warnf("failed to open %s: %v", target.Path, err)
					_ = r.Close()
					return
				}

				bar := BytesBar(f.Size-target.Size, f.ID)
				if _, err := r.Seek(target.Size, 0); err != nil {
					log.Warnf("failed to seek %s: %v", f.Path, err)
					_ = r.Close()
					_ = w.Close()
					return
				}

				// create proxy reader
				reader := bar.ProxyReader(r)
				_, err = io.Copy(w, reader)
				_ = reader.Close()
				_ = w.Close()
				_ = r.Close()
			}

		}()
	}

	for i, f := range fs {
		f.ID = fmt.Sprintf("[%d/%d] %s", i+1, len(fs), f.Name())
		taskChan <- f
	}

	close(taskChan)
	defer progress.Wait()
}
