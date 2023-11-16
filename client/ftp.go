package client

import (
	"crypto/md5"
	"fmt"
	"github.com/jlaffaye/ftp"
	"github.com/ygidtu/transfer/base/fi"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FtpClient 连接的配置
type FtpClient struct {
	Host   *Proxy
	Client *ftp.ServerConn
}

/*
NewFtp 新建新的ftp客户端
@host: ftp的地址
*/
func NewFtp(host *Proxy) *FtpClient {
	if host.Port == "" {
		host.Port = "21"
	}

	cfg := &FtpClient{Host: host}

	if err := cfg.connect(); err != nil {
		log.Fatal(err)
	}
	return cfg
}

// clientType 返回客户端类型
func (_ *FtpClient) clientType() transferClientType { return Ftp }

// connect 连接至ftp服务器
func (fc *FtpClient) connect() error {
	c, err := ftp.Dial(fc.Host.Addr(), ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		return err
	}

	if fc.Host.Username != "" && fc.Host.Password != "" {
		err = c.Login(fc.Host.Username, fc.Host.Password)
		if err != nil {
			return err
		}
	}

	fc.Client = c
	return nil
}

// close 退出ftp
func (fc *FtpClient) close() error {
	return fc.Client.Quit()
}

// listFiles 列出ftp上特定目录下的所有文件
func (fc *FtpClient) listFiles(file *File) (FileList, error) {
	files := FileList{Files: []*File{}}

	stat, err := fc.stat(file.Path)
	if err != nil {
		return files, err
	}

	if !stat.IsDir() {
		f, err := fc.newFile(file.Path)
		if err != nil {
			return files, err
		}
		files.Files = append(files.Files, f)
		files.Total += f.Size
	} else {
		// walk a directory
		walker := fc.Client.Walk(file.Path)

		for walker.Next() {
			e := walker.Stat()
			if opt.Skip && e.Name != "." && e.Name != "./" && strings.HasPrefix(e.Name, ".") {
				continue
			}
			if e.Type == ftp.EntryTypeFile {
				files.Files = append(
					files.Files,
					&File{Path: e.Name, Size: int64(e.Size), IsFile: true, client: fc},
				)
				files.Total += int64(e.Size)
			} else if e.Type == ftp.EntryTypeLink {
				files.Files = append(
					files.Files,
					&File{Path: e.Name, Size: int64(e.Size), IsFile: true, client: fc},
				)
				files.Total += int64(e.Size)
			}
		}

		if walker.Err() != nil {
			return files, walker.Err()
		}
	}
	return files, nil
}

// exists check whether file or directory exists
func (fc *FtpClient) exists(path string) bool {
	_, err := fc.stat(path)
	return !os.IsNotExist(err)
}

// newFile 新建新的ftp文件对象
func (fc *FtpClient) newFile(path string) (*File, error) {
	var err error
	stat, err := fc.Client.List(path)

	if len(stat) > 0 && err == nil {
		for _, i := range stat {
			if i.Name == filepath.Base(path) {
				return &File{Path: path, Size: int64(i.Size), IsFile: i.Type == ftp.EntryTypeFile, client: fc}, nil
			}
		}
	}

	if path != "/" {
		err = os.ErrNotExist
	}

	return &File{Path: path, Size: 0, client: fc, IsFile: true}, err
}

// mkdir as name says
func (fc *FtpClient) mkdir(path string) error {
	_, err := fc.stat(path)
	if err != nil {
		return fc.Client.MakeDir(path)
	}
	return nil
}

// MkParent make parent directory of path
func (fc *FtpClient) mkParent(path string) error {
	return fc.mkdir(filepath.Dir(path))
}

/*
reader 提供ftp服务器上特定文件的reader
@path: 文件路径
@offset: 文件的特定位置开始读取
*/
func (fc *FtpClient) reader(path string, offset int64) (io.ReadCloser, error) {
	if ok := fc.exists(path); !ok {
		return nil, os.ErrNotExist
	}
	log.Debugf("FTP retr %v from %v", path, offset)
	return fc.Client.RetrFrom(path, uint64(offset))
}

/*
writeAt 向服务器上某个文件的特定位置写入数据
@reader: 源文件的reader
@path: 写入对象的地址
@trunc: 写入的模式为trunc还是append
*/
func (fc *FtpClient) writeAt(reader io.Reader, path string, trunc bool) error {
	offset := 0
	if !trunc {
		stat, err := fc.stat(path)
		if err == nil {
			offset = int(stat.Size())
		}
	}
	return fc.Client.StorFrom(path, reader, uint64(offset))
}

/*
getMd5 获取服务器上小文件的完整md5和大文件的头尾md5
@file: 服务器上文件对象
*/
func (fc *FtpClient) getMd5(file *File) error {
	if stat, err := fc.stat(file.Path); !os.IsNotExist(err) {
		var data []byte
		r, err := fc.reader(file.Path, 0)
		if err != nil {
			return err
		}
		if stat.Size() < fileSizeLimit {
			data, err = io.ReadAll(r)
			if err := r.Close(); err != nil {
				return err
			}
		} else {
			data = make([]byte, capacity)
			_, err = r.Read(data[:capacity/2])
			if err != nil {
				return err
			}
			if err := r.Close(); err != nil {
				return err
			}
			r, err = fc.reader(file.Path, stat.Size()-capacity/2)
			if err != nil {
				return err
			}
			if err := r.Close(); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		file.Md5 = fmt.Sprintf("%x", md5.Sum(data))
	}
	return nil
}

/*
stat 获取服务器上特定文件信息
@path: 文件路径
*/
func (fc *FtpClient) stat(path string) (fs.FileInfo, error) {
	stat, err := fc.Client.List(path)
	if len(stat) > 0 && err == nil {
		for _, i := range stat {
			if i.Name == filepath.Base(path) {
				return fi.FtpFileInfo{File: i}, nil
			}
		}
	}
	if path == "/" {
		return fi.FtpFileInfo{Root: true}, nil
	}
	return nil, os.ErrNotExist
}
