package client

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalClient 本地文件的客户端，无需任何配置
type LocalClient struct{}

// NewLocal 新建本地客户端
func NewLocal() *LocalClient {
	return &LocalClient{}
}

// clientType 返回客户端类型
func (_ *LocalClient) clientType() transferClientType {
	return Local
}

// connect 仅实现client接口，该函数在local客户端上无实际意义
func (l *LocalClient) connect() error { return nil }

// close 仅实现client接口，该函数在local客户端上无实际意义
func (l *LocalClient) close() error { return nil }

/*
getMd5 计算本地小文件的完成md5，大文件的头尾md5
@file: 本地文件的路径
*/
func (l *LocalClient) getMd5(file *File) error {
	stat, err := os.Stat(file.Path)
	if os.IsNotExist(err) {
		return err
	}
	var data []byte
	// 文件小于10M
	f, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	if stat.Size() < fileSizeLimit {
		data, err = io.ReadAll(f)
		if err != nil {
			return err
		}
	} else {
		// 文件大于10M，则从头尾各取一部分机选MD5
		data = make([]byte, capacity)
		f, err := os.Open(file.Path)

		_, err = f.Read(data[:capacity/2])
		if err != nil {
			return err
		}
		_, err = f.ReadAt(data[capacity/2:], stat.Size()-capacity/2)
		if err != nil {
			return err
		}
	}

	if data != nil {
		file.Md5 = fmt.Sprintf("%x", md5.Sum(data))
	}
	return nil
}

/*
mkdir 新建本地文件夹
@path: 本地文件的路径
*/
func (l *LocalClient) mkdir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return nil
}

/*
mkParent 新建本地文件的父目录
@path: 本地文件的路径
*/
func (l *LocalClient) mkParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), os.ModePerm)
}

/*
newFile 生成本地文件对象
@path: 本地文件的路径
*/
func (l *LocalClient) newFile(path string) (*File, error) {
	stat, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), client: l}, nil
	}
	return &File{Path: path, Size: 0, IsFile: false, client: l}, nil
}

/*
exists 检查本地文件是否存在
@path: 本地文件的路径
*/
func (l *LocalClient) exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

/*
reader 本地文件的reader
@path: 本地文件的路径
@offset: 本地文件的起始读取位置
*/
func (l *LocalClient) reader(path string, offset int64) (io.ReadCloser, error) {
	r, err := os.Open(path)
	if err != nil {
		return r, err
	}
	_, err = r.Seek(offset, 0)
	return r, err
}

/*
readSeeker 本地文件的reader，但返回io.ReadSeekCloser
@path: 本地文件的路径
*/
func (l *LocalClient) readSeeker(path string) (io.ReadSeekCloser, error) {
	r, err := os.Open(path)
	if err != nil {
		return r, err
	}
	return r, err
}

/*
writeAt 本地文件的writer
@reader: 源文件的reader
@path: 本地文件的路径
@trucn: 本地文件是以trunc还是append模式打开
*/
func (l *LocalClient) writeAt(reader io.Reader, path string, trunc bool) error {
	writerCode := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if trunc {
		writerCode = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	f, err := os.OpenFile(path, writerCode, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	return err
}

/*
stat 返回本地文件的文件信息，直接调用os.Stat
@path: 本地文件的路径
*/
func (l *LocalClient) stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

/*
listFiles 返回本地目录下的所有文件
@file: 本地文件的路径
*/
func (l *LocalClient) listFiles(file *File) (FileList, error) {
	files := FileList{Files: []*File{}, Total: 0}
	var err error

	if file.IsFile {
		files.Files = append(files.Files, file)
		files.Total += file.Size
	} else if file.Path != "" {
		err = filepath.Walk(file.Path, func(p string, info os.FileInfo, err error) error {
			if opt.Skip && info.Name() != "./" && info.Name() != "." {
				if strings.HasPrefix(info.Name(), ".") {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if info != nil && !info.IsDir() {
				files.Files = append(files.Files, &File{
					Path:   p,
					Size:   info.Size(),
					IsFile: !info.IsDir(),
					client: l,
				})

				files.Total += info.Size()
			}
			return nil
		})
	}

	return files, err
}
