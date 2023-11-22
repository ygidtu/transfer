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

// TransferClientType 自定义的客户端类型
type TransferClientType string

const (
	NoClient = "no-client"
	Aws      = "aws-s3"
	Sftp     = "sftp"
	Ftp      = "ftp"
	Http     = "http-client"
	HttpS    = "http-server"
	Local    = "local-client"
)

// Client 客户端的接口
type Client interface {
	clientType() TransferClientType                          // 返回客户端类型
	connect() error                                          // 实现客户端的链接功能
	close() error                                            // 关闭客户端
	listFiles(src *File) (FileList, error)                   // 列出特定目录下的所有文件
	exists(path string) bool                                 // 文件或文件夹是否存在
	newFile(path string) (*File, error)                      // 根据客户端类型新建新的File
	mkdir(path string) error                                 // 新建新的目录
	mkParent(path string) error                              // 新建新的父目录
	getMd5(file *File) error                                 // 获取不同客户端上文件的md5
	reader(path string, offset int64) (io.ReadCloser, error) // 不同客户端上文件的reader
	writeAt(reader io.Reader, path string, trunc bool) error // 在不同客户端文件的特定位置写入数据
	stat(path string) (os.FileInfo, error)                   // 获取不同客户端上的文件信息
}

/*
clientFromProxy 根据host和proxy信息新建对应的客户端
@host: 客户端的地址
@proxy: 客户端的代理
@bucket: aws特定的bucket信息，可选项
*/
func clientFromProxy(host, proxy *Proxy, bucket string) (Client, error) {
	var client Client
	var err error
	if host.Scheme == "file" {
		return NewLocal(), nil
	} else if host.Scheme == "ssh" {
		client = NewSftp(host, proxy, opt.IdRsa, opt.Scp, opt.Concurrent)
	} else if host.Scheme == "ftp" {
		client = NewFtp(host)
	} else if strings.Contains(host.Scheme, "http") {
		client, err = NewHTTPClient(host, proxy)
	} else if host.Scheme == "s3" {
		client, err = NewS3Client(host, proxy, bucket)
	}

	if client == nil {
		return nil, fmt.Errorf("unsupported scheme = %s", host.Scheme)
	} else if err != nil {
		return nil, err
	}

	err = client.connect()
	return client, err
}

/*
InitClient 初始化客户端
@option: 命令行参数
*/
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

	sourceClient, err := clientFromProxy(sourceP, proxy, opt.Bucket)
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
		serverClient, err := clientFromProxy(server, proxy, "")
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
		targetClient, err := clientFromProxy(targetP, proxy, opt.Bucket)
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
