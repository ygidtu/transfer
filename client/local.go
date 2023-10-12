package client

import (
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
	return file.GetMd5()
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
	return &File{
		Path: path, Size: stat.Size(),
		IsFile: !stat.IsDir(), IsLocal: true,
	}, err
}

func (l *LocalClient) Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (l *LocalClient) Reader(path string) (io.ReadSeekCloser, error) {
	return os.Open(path)
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
		log.Infof("List files from %s", file.Path)

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
					Path:    p,
					Size:    info.Size(),
					IsFile:  !info.IsDir(),
					IsLocal: true})

				files.Total += info.Size()
			}
			return nil
		})
	}

	return files, err
}
