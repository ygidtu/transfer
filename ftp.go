package main

import (
	"fmt"
	"github.com/jlaffaye/ftp"
	"github.com/schollz/progressbar/v3"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FtpConfig 连接的配置
type FtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

func newFtp(host string) *FtpConfig {
	cfg := &FtpConfig{Port: "21"}
	hosts := strings.Split(host, "@")

	if len(hosts) < 2 {
		hosts = strings.Split(hosts[0], ":")
		cfg.Host = hosts[0]
		if len(hosts) > 1 {
			cfg.Password = hosts[1]
		}
		return cfg
	}

	users := strings.Split(hosts[0], ":")
	cfg.Username = users[0]
	if len(users) > 1 {
		cfg.Password = users[1]
	}

	hosts = strings.Split(hosts[1], ":")
	cfg.Host = hosts[0]
	if len(hosts) > 1 {
		cfg.Port = hosts[1]
	}
	return cfg
}

func ftpMkdir(c *ftp.ServerConn, path string) error {
	_, err := c.List(path)
	if err != nil {
		return c.MakeDir(path)
	}
	return nil
}

func makedir(path string) error {
	if _, ok := os.Stat(path); os.IsNotExist(ok) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return nil
}

func process(task *Task, c *ftp.ServerConn, pull bool) {
	f := task.Source
	target := task.Target

	if pull {
		if err := makedir(filepath.Dir(target)); err != nil {
			log.Warnf("failed to mkdir %s at local: %v", filepath.Dir(target), err)
			return
		}

		size := int64(0)
		if stat, err := os.Stat(f.Path); !os.IsNotExist(err) {
			size = stat.Size()
		}

		if size > f.Size {
			log.Warnf("local file seems corrupted")
			if err := os.Remove(target); err != nil {
				log.Warnf("failed to delete lcoal file %s: %v", target, err)
				return
			}
		}

		//bar := BytesBar(f.Size, filepath.Base(f.Path))
		w, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			log.Warnf("failed to open local file %s: %v", f.Path, err)
			return
		}

		if f.Size > size {
			log.Infof("%s <- %s [restore from: %s]", target, f.Path, ByteCountDecimal(size))
		} else {
			log.Infof("%s <- %s", target, f.Path)
		}

		resp, err := c.RetrFrom(f.Path, uint64(size))
		if err != nil {
			log.Warnf("failed to open file %s from remote: %v", f.Path, err)
		}

		bar := BytesBar(size, task.ID)
		if _, err = io.Copy(io.MultiWriter(w, bar), resp); err != nil {
			log.Warnf("failed to get file from remote: %v", err)
		}
		_ = bar.Finish()
		_ = w.Close()
	} else {
		if f.Path != path {
			f.Path = filepath.Join(path, f.Path)
		}

		if err := ftpMkdir(c, filepath.Dir(target)); err != nil {
			log.Warnf("failed to mkdir %s at remote: %v", filepath.Dir(target), err)
			return
		}

		offset, err := c.FileSize(target)
		if err != nil {
			offset = 0
		}

		if offset > f.Size {
			log.Warnf("remote file seems corrupted")
			if err = c.Delete(target); err != nil {
				log.Warnf("failed to delete remote file %s: %v", target, err)
				return
			}
		}

		bar := BytesBar(f.Size, filepath.Base(f.Path))
		r, err := os.Open(f.Path)
		if err != nil {
			log.Warnf("failed to open local file %s: %v", f.Path, err)
			return
		}

		if offset > 0 {
			log.Infof("%s -> %s [restore from: %s]", f.Path, target, ByteCountDecimal(offset))
			if _, err := r.Seek(offset, 0); err != nil {
				log.Warnf("failed to seek %s: %v", f.Path, err)
				return
			}
		} else {
			log.Infof("%s -> %s", f.Path, target)
		}

		reader := progressbar.NewReader(r, bar)
		err = c.StorFrom(target, &reader, uint64(offset))
		if err != nil {
			log.Warnf("failed to put file to remote: %v", err)
		}
		_ = bar.Finish()
		_ = reader.Close()
		_ = r.Close()
	}
}

func initFtp(host, remote string, pull bool) {

	cfg := newFtp(host)
	log.Infof("Connect to %s:%s", cfg.Host, cfg.Port)
	c, err := ftp.Dial(fmt.Sprintf("%s:%s", cfg.Host, cfg.Port), ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Username != "" && cfg.Password != "" {
		err = c.Login(cfg.Username, cfg.Password)
		if err != nil {
			log.Fatal(err)
		}
	}

	if pull {
		target, err := c.List(remote)
		if err != nil {
			log.Fatal(err)
		}

		for idx, f := range target {
			task := &Task{
				&File{Path: filepath.Join(remote, filepath.Base(f.Name)), Size: int64(f.Size)},
				filepath.Join(path, filepath.Base(f.Name)),
				fmt.Sprintf("[%d/%d]%s", idx+1, len(target), filepath.Base(f.Name))}
			process(task, c, pull)
		}
	} else {
		target, err := listFiles()
		if err != nil {
			log.Fatal(err)
		}

		for idx, f := range target {
			task := &Task{f, filepath.Join(remote, f.Path), fmt.Sprintf("[%d/%d]%s", idx+1, len(target), filepath.Base(f.Path))}
			if f.Path == path {
				task = &Task{f, filepath.Join(remote, f.Name()), fmt.Sprintf("[%d/%d]%s", idx+1, len(target), filepath.Base(f.Path))}
			}
			process(task, c, pull)
		}

	}
	if err = c.Quit(); err != nil {
		log.Fatalf("failed to quite ftp: %v", err)
	}
}
