package client

import (
	"fmt"
	"github.com/schollz/progressbar/v3"
	"github.com/ygidtu/transfer/base"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	log           *zap.SugaredLogger
	opt           *base.Options
	capacity      = int64(2000)
	fileSizeLimit = int64(10 * 1024 * 1024)
)

type Client interface {
	Connect() error
	Close() error
	ListFiles(src *File) (FileList, error)
	Exists(path string) bool
	NewFile(path string) (*File, error)
	Mkdir(path string) error
	MkParent(path string) error
	GetMd5(file *File) error
	Reader(path string) (io.ReadSeekCloser, error)
	Writer(path string, code int) (io.WriteCloser, error)
	Stat(path string) (os.FileInfo, error)
}

func InitClient(option *base.Options, bar *progressbar.ProgressBar) (*Transfer, error) {
	log = base.SugaredLog
	opt = option

	transfer := &Transfer{bar: bar, concurrent: option.Concurrent}
	if opt.Pull {
		transfer.mode = R2L
	} else {
		transfer.mode = L2R
	}

	log.Infof("Running mode = %v", opt.Verbs)
	switch opt.Verbs {
	case "cp":
		transfer.client = NewLocal()
		transfer.mode = L2R

		src, err := NewFile(opt.Copy.Path)
		if err != nil {
			return nil, err
		}
		dst, _ := NewFile(opt.Copy.Remote)
		transfer.local = src
		transfer.remote = dst
	case "sftp":
		{
			if !strings.HasPrefix(opt.Sftp.Host, "ssh") {
				opt.Sftp.Host = fmt.Sprintf("ssh://%s", opt.Sftp.Host)
			}

			if opt.Sftp.Path == "" {
				opt.Sftp.Path = "./"
			}

			transfer.client = NewSftp(opt.Sftp.Host, opt.Sftp.Proxy, opt.Sftp.IdRsa, opt.Sftp.Scp, opt.Concurrent)
			src, err := NewFile(opt.Sftp.Path)
			if err != nil {
				return nil, err
			}
			dst, err := transfer.NewFile(opt.Sftp.Remote)
			if err != nil {
				return nil, err
			}
			transfer.local = src
			transfer.remote = dst
		}
	case "ftp":
		{
			if opt.Ftp.Path == "" {
				opt.Ftp.Path = "./"
			}

			if abs, err := filepath.Abs(opt.Ftp.Path); err != nil {
				log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Ftp.Path, err)
			} else {
				opt.Ftp.Path = abs
			}

			transfer.client = NewFtp(opt.Ftp.Host)

			src, err := NewFile(opt.Ftp.Path)
			if err != nil {
				return nil, err
			}
			dst, _ := transfer.NewFile(opt.Ftp.Remote)
			transfer.local = src
			transfer.remote = dst
		}
	case "http":
		{
			if opt.Ftp.Path == "" {
				opt.Ftp.Path = "./"
			}

			if abs, err := filepath.Abs(opt.Ftp.Path); err != nil {
				log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Ftp.Path, err)
			} else {
				opt.Http.Path = abs
			}

			client1, err := NewHTTPClient(opt)
			log.Fatal(err)
			transfer.client = client1

			src, err := NewFile(opt.Http.Path)
			if err != nil {
				return nil, err
			}
			dst, err := transfer.NewFile(opt.Http.Path)
			if err != nil {
				return nil, err
			}
			transfer.local = src
			transfer.remote = dst
		}
	}

	return transfer, nil
}
