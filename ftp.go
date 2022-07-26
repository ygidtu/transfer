package main

import (
	"fmt"
	"github.com/jlaffaye/ftp"
	"github.com/schollz/progressbar/v3"
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

		bar := BytesBar(source.Size, source.Name())
		defer bar.Finish()
		r, err := os.Open(source.Path)
		if err != nil {
			return fmt.Errorf("failed to open local file %s: %v", source.Path, err)
		}
		defer r.Close()

		if offset > 0 {
			log.Infof("%s -> %s [restore from: %s]", source.Path, target.Path, ByteCountDecimal(offset))
			if _, err := r.Seek(offset, 0); err != nil {
				return fmt.Errorf("failed to seek %s: %v", source.Path, err)
			}
		} else {
			log.Infof("%s -> %s", source.Path, target.Path)
		}

		reader := progressbar.NewReader(r, bar)
		defer reader.Close()
		err = fc.Client.StorFrom(target.Path, &reader, uint64(offset))
		if err != nil {
			log.Warnf("failed to put file to remote: %v", err)
		}
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
		defer w.Close()

		if source.Size > size {
			log.Infof("%s <- %s [restore from: %s]", target.Path, source.Path, ByteCountDecimal(size))
		} else {
			log.Infof("%s <- %s", target.Path, source.Path)
		}

		resp, err := fc.Client.RetrFrom(source.Path, uint64(size))
		if err != nil {
			log.Warnf("failed to open file %s from remote: %v", source.Path, err)
		}

		bar := BytesBar(size, source.Name())
		defer bar.Finish()
		if _, err = io.Copy(io.MultiWriter(w, bar), resp); err != nil {
			log.Warnf("failed to get file from remote: %v", err)
		}

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
	if opt.Ftp.Pull {
		fs, err := ListFilesFtp(client, opt.Ftp.Remote)
		if err != nil {
			log.Fatal(err)
		}

		for _, f := range fs {
			if err := client.Pull(f, f.GetTarget(opt.Ftp.Remote, opt.Ftp.Path)); err != nil {
				log.Warn(err)
			}
		}
	} else {
		root, err := NewFile(opt.Ftp.Path)
		if err != nil {
			log.Fatal(err)
		}

		fs, err := ListFilesLocal(root)
		if err != nil {
			log.Fatal(err)
		}

		for _, f := range fs {
			if err := client.Pull(f, f.GetTarget(opt.Ftp.Path, opt.Ftp.Remote)); err != nil {
				log.Warn(err)
			}
		}
	}
}
