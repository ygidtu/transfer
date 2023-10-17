package client

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalClient struct{}

func NewLocal() *LocalClient {
	return &LocalClient{}
}

func (l *LocalClient) Connect() error                        { return nil }
func (l *LocalClient) Close() error                          { return nil }
func (l *LocalClient) RemoteToRemote(_ *File, _ *File) error { return nil }

func (l *LocalClient) GetMd5(file *File) error {
	stat, err := os.Stat(file.Path)
	if os.IsNotExist(err) {
		return err
	}
	var data []byte
	// 文件小于10M

	if stat.Size() < fileSizeLimit {
		log.Debugf("file size < 10M")
		f, err := os.Open(file.Path)
		if err != nil {
			return err
		}
		data, err = io.ReadAll(f)
		if err != nil {
			return err
		}
	} else {
		// 文件大于10M，则从头尾各取一部分机选MD5
		log.Debugf("file size >= 10M")

		// 设定取值的大小
		data = make([]byte, capacity)
		f, err := os.Open(file.Path)
		if err != nil {
			return err
		}
		defer f.Close()

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

func (l *LocalClient) Mkdir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return nil
}

func (l *LocalClient) MkParent(path string) error {
	return os.MkdirAll(filepath.Dir(path), os.ModePerm)
}

func (l *LocalClient) NewFile(path string) (*File, error) {
	stat, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), client: l}, nil
	}
	return &File{Path: path, Size: 0, IsFile: false, client: l}, nil
}

func (l *LocalClient) Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (l *LocalClient) Reader(path string, offset int64) (io.ReadCloser, error) {
	r, err := os.Open(path)
	if err != nil {
		return r, err
	}
	_, err = r.Seek(offset, 0)
	return r, err
}

func (l *LocalClient) Writer(path string, code int) (io.WriteCloser, error) {
	return os.OpenFile(path, code, os.ModePerm)
}

func (l *LocalClient) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (l *LocalClient) ListFiles(file *File) (FileList, error) {
	files := FileList{Files: []*File{}}
	var err error

	if file.IsFile {
		files.Files = append(files.Files, file)
		files.Total += file.Size
	} else {
		err = filepath.Walk(file.Path, func(p string, info os.FileInfo, err error) error {
			if opt.Skip && info.Name() != "./" && info.Name() != "." {
				if strings.HasPrefix(info.Name(), ".") {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if !info.IsDir() {
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
