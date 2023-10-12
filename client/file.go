package client

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// File is used to kept file path and size
type File struct {
	Path    string
	Size    int64
	Target  string
	IsFile  bool
	IsLink  bool
	IsLocal bool
	ID      string
	Md5     string
}

type FileList struct {
	Files []*File
	Total int64
}

// Name is used to return the file name
func (file *File) Name() string {
	return filepath.Base(file.Path)
}

// CheckParent is used to create the parent path of file if not exists, only work with local file
func (file *File) CheckParent() error {
	if file.IsLocal {
		parent := filepath.Dir(file.Path)
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			err = os.MkdirAll(parent, os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to mkdir remove directory: %s - %v", parent, err)
			}
		}
		return nil
	}
	return fmt.Errorf("do not support check remote parents")
}

// GetTarget generate the target path
func (file *File) GetTarget(source, target *File) *File {
	sourcePath := source.Path
	if source.IsFile {
		sourcePath = filepath.Dir(source.Path)
	}
	path := strings.Replace(file.Path, sourcePath, "", 1)
	path = strings.TrimLeft(path, "/")

	return &File{Path: filepath.Join(target.Path, path), IsLocal: !file.IsLocal, IsFile: file.IsFile}
}

// GetMd5 calculate the md5 hash of partial file
func (file *File) GetMd5() error {
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

// NewFile create new File object
func NewFile(path string) (*File, error) {
	stat, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), IsLocal: true}, nil
	}
	return &File{Path: path, Size: 0, IsFile: false, IsLocal: true}, err
}

// NewFileCreate create new File object, if file is not exists, create an empty file
func NewFileCreate(path string) (file *File, err error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
	}

	if err == nil {
		stat, _ = os.Stat(path)
		file = &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), IsLocal: true}
	}

	return file, err
}
