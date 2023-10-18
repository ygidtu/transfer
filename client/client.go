package client

import (
	"fmt"
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
	Reader(path string, offset int64) (io.ReadCloser, error)
	WriteAt(reader io.Reader, path string, trunc bool) error
	Stat(path string) (os.FileInfo, error)
}

func clientFromProxy(host, proxy *Proxy) (Client, error) {
	if host.Scheme == "file" {
		return NewLocal(), nil
	} else if host.Scheme == "ssh" {
		return NewSftp(host, proxy, opt.IdRsa, opt.Scp, opt.Concurrent), nil
	} else if host.Scheme == "ftp" {
		return NewFtp(host), nil
	} else if strings.Contains(host.Scheme, "http") {
		return NewHTTPClient(host, proxy)
	}
	return nil, fmt.Errorf("unsupported scheme = %s", host.Scheme)
}

func InitClient(option *base.Options) (*Transfer, error) {
	log = base.SugaredLog
	opt = option
	var err error

	var proxy *Proxy
	if opt.Proxy != "" {
		proxy, err = CreateProxy(opt.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to decode proxy: %v", err)
		}
	}

	if !strings.Contains(opt.Source, "://") {
		abs, err := filepath.Abs(opt.Source)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for: %s", opt.Source)
		}
		opt.Source = fmt.Sprintf("file://%s", abs)
	}

	sourceP, err := CreateProxy(opt.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source path: %v", err)
	}

	sourceClient, err := clientFromProxy(sourceP, proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for source path: %v", err)
	}

	source, err := NewFile(sourceP.Path, sourceClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get source file/directory: %v", err)
	}

	if opt.Server != "" {
		if !strings.HasPrefix(opt.Server, "http") {
			opt.Server = fmt.Sprintf("http://%s", opt.Server)
		}
		server, err := CreateProxy(opt.Server)
		if err != nil {
			return nil, fmt.Errorf("failed to decode server host url: %v", err)
		}
		serverClient, err := clientFromProxy(server, proxy)
		source, err = NewFile(sourceP.Path, serverClient)

		return &Transfer{concurrent: option.Concurrent, source: source}, nil
	} else {
		if opt.Target == "" {
			opt.Target = opt.Source
		}

		if !strings.Contains(opt.Target, "://") {
			abs, err := filepath.Abs(opt.Target)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for: %s", opt.Target)
			}
			opt.Target = fmt.Sprintf("file://%s", abs)
		}
		targetP, err := CreateProxy(opt.Target)
		if err != nil {
			return nil, fmt.Errorf("failed to decode target path: %v", err)
		}
		targetClient, err := clientFromProxy(targetP, proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for target path: %v", err)
		}
		target, err := NewFile(targetP.Path, targetClient)
		if err != nil {
			return nil, fmt.Errorf("failed to get target file/directory: %v", err)
		}

		return &Transfer{concurrent: option.Concurrent, source: source, target: target}, nil
	}
}
