package main

import (
	"fmt"
	"github.com/jlaffaye/ftp"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FtpClient 连接的配置
type FtpClient struct {
	Host   *Proxy
	Client *ftp.ServerConn
}

func NewFtp(host string) *FtpClient {
	remoteHost, err := CreateProxy(host)
	if err != nil {
		log.Fatalf("wrong format of ssh server [%s]:  %s", host, err)
	}

	if remoteHost.Port == "" {
		remoteHost.Port = "21"
	}

	cfg := &FtpClient{Host: remoteHost}

	c, err := ftp.Dial(cfg.Host.Addr(), ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Host.Username != "" && cfg.Host.Password != "" {
		err = c.Login(cfg.Host.Username, cfg.Host.Password)
		if err != nil {
			log.Fatal(err)
		}
	}
	cfg.Client = c
	return cfg
}

func (fc *FtpClient) NewFile(path string) (*File, error) {
	walker := fc.Client.Walk(path)

	if os.IsNotExist(walker.Err()) {
		if err := fc.Client.MakeDir(path); err != nil {
			return nil, err
		}
	}

	walker = fc.Client.Walk(path)
	if walker.Err() != nil {
		return nil, walker.Err()
	}

	return &File{
		Path: path, Size: int64(walker.Stat().Size),
		IsFile: walker.Stat().Type == ftp.EntryTypeFile, IsLocal: false,
	}, nil
}

func (fc *FtpClient) Put(source, target *File) error {
	if source.IsLocal && !target.IsLocal {

		if err := fc.Mkdir(filepath.Dir(target.Path)); err != nil {
			return fmt.Errorf("failed to mkdir %s at remote: %v", filepath.Dir(target.Path), err)
		}

		offset, err := fc.Client.FileSize(target.Path)
		if err != nil {
			offset = 0
		}

		if offset > source.Size {
			log.Warnf("remote file seems corrupted")
			if err = fc.Client.Delete(target.Path); err != nil {
				return fmt.Errorf("failed to delete remote file %s: %v", target.Path, err)
			}
		}

		bar.Describe(source.ID)
		r, err := os.Open(source.Path)
		if err != nil {
			return fmt.Errorf("failed to open local file %s: %v", source.Path, err)
		}

		if offset > 0 {
			log.Infof("%s -> %s [restore from: %s]", source.Path, target.Path, ByteCountDecimal(offset))
			if _, err := r.Seek(offset, 0); err != nil {
				return fmt.Errorf("failed to seek %s: %v", source.Path, err)
			}
		} else if source.Size == offset {
			log.Infof("Skip: %s", source.Path)
			_ = bar.Add64(offset)
			return nil
		} else {
			log.Infof("%s -> %s", source.Path, target.Path)
		}

		err = fc.Client.StorFrom(target.Path, io.MultiReader(r, bar), uint64(offset))
		if err != nil {
			log.Warnf("failed to put file to remote: %v", err)
		}
		_ = bar.Finish()
		_ = r.Close()
		return nil
	}

	return fmt.Errorf("soure file [%v] should be local, target file [%v] should be remote", source, target)
}

func (fc *FtpClient) Pull(source, target *File) error {
	if !source.IsLocal && target.IsLocal {
		if err := target.CheckParent(); err != nil {
			return err
		}

		size := int64(0)
		if stat, err := os.Stat(target.Path); !os.IsNotExist(err) {
			size = stat.Size()
		}

		if size > source.Size {
			log.Warnf("local file seems corrupted")
			if err := os.Remove(target.Path); err != nil {
				return fmt.Errorf("failed to delete lcoal file %s: %v", target.Path, err)
			}
		}

		//bar := BytesBar(f.Size, filepath.Base(f.Path))
		w, err := os.OpenFile(target.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to open local file %s: %v", target.Path, err)
		}

		if source.Size > size {
			log.Infof("%s <- %s [restore from: %s]", target.Path, source.Path, ByteCountDecimal(size))
		} else if source.Size == size {
			_ = bar.Add64(source.Size)
			log.Infof("Skip: %s", source.Path)
			return nil
		} else {
			log.Infof("%s <- %s", target.Path, source.Path)
		}

		resp, err := fc.Client.RetrFrom(source.Path, uint64(size))
		if err != nil {
			log.Warnf("failed to open file %s from remote: %v", source.Path, err)
		}

		bar.Describe(source.ID)
		if _, err = io.Copy(io.MultiWriter(w, bar), resp); err != nil {
			log.Warnf("failed to get file from remote: %v", err)
		}

		_ = resp.Close()
		_ = w.Close()
		_ = bar.Finish()
		return nil
	}

	return fmt.Errorf("soure file [%v] should be remote, target file [%v] should be local", source, target)
}

func (fc *FtpClient) Mkdir(path string) error {
	_, err := fc.Client.List(path)
	if err != nil {
		return fc.Client.MakeDir(path)
	}
	return nil
}

func initFtp(opt *options) {
	if opt.Ftp.Path == "" {
		opt.Ftp.Path = "./"
	}

	if abs, err := filepath.Abs(opt.Ftp.Path); err != nil {
		log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Ftp.Path, err)
	} else {
		opt.Ftp.Path = abs
	}

	client := NewFtp(opt.Ftp.Host)
	files := make([]*File, 0, 0)
	if opt.Ftp.Pull {
		fs, err := ListFilesFtp(client, opt.Ftp.Remote)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	} else {
		root, err := NewFile(opt.Ftp.Path)
		if err != nil {
			log.Fatal(err)
		}

		fs, err := ListFilesLocal(root)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	}

	taskChan := make(chan *File)

	for i := 0; i < opt.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				f, ok := <-taskChan

				if !ok {
					break
				}

				if opt.Ftp.Pull {
					if root, err := NewFile(opt.Ftp.Path); err != nil {
						log.Fatal(err)
					} else {
						target = root
					}

					if root, err := client.NewFile(opt.Ftp.Remote); err != nil {
						log.Fatal(err)
					} else {
						source = root
					}

					if err := client.Pull(f, f.GetTarget(source, target)); err != nil {
						log.Warn(err)
					}
				} else {
					if root, err := NewFile(opt.Ftp.Path); err != nil {
						log.Fatal(err)
					} else {
						source = root
					}

					if root, err := client.NewFile(opt.Ftp.Remote); err != nil {
						log.Fatal(err)
					} else {
						target = root
					}

					if err := client.Put(f, f.GetTarget(source, target)); err != nil {
						log.Warn(err)
					}
				}
			}
		}()
	}

	close(taskChan)
}
