package client

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// File is used to kept file path and size
type File struct {
	Path   string
	Size   int64
	Target string
	IsFile bool
	IsLink bool
	ID     string
	Md5    string
	client Client
}

type FileList struct {
	Files []*File
	Total int64
}

// Name is used to return the file name
func (file *File) Name() string {
	return filepath.Base(file.Path)
}

// GetTarget generate the target path
func (file *File) GetTarget(source, target *File) *File {
	sourcePath := source.Path
	if source.IsFile {
		sourcePath = filepath.Dir(source.Path)
	}
	path := strings.Replace(file.Path, sourcePath, "", 1)
	path = strings.TrimLeft(path, "/")

	dst := &File{Path: filepath.Join(target.Path, path), IsFile: source.IsFile, client: target.client}

	stat, _ := dst.Stat()
	if stat != nil {
		dst.Size = stat.Size()
		dst.IsFile = !stat.IsDir()
	}

	return dst
}

// NewFile create new File object
func NewFile(path string, client Client) (*File, error) {
	return client.NewFile(path)
}

func (file *File) Reader(offset int64) (io.ReadCloser, error) {
	return file.client.Reader(file.Path, offset)
}

func (file *File) WriteAt(reader io.Reader, trunc bool) error {
	return file.client.WriteAt(reader, file.Path, trunc)
}

// GetMd5 calculate the md5 hash of partial file
func (file *File) GetMd5() error {
	return file.client.GetMd5(file)
}

func (file *File) Exists() bool {
	return file.client.Exists(file.Path)
}

func (file *File) MkParent() error {
	return file.client.MkParent(file.Path)
}

func (file *File) Stat() (os.FileInfo, error) {
	return file.client.Stat(file.Path)
}

func (file *File) Children() (FileList, error) {
	fs := FileList{Files: []*File{}}
	stat, err := file.Stat()
	if err != nil {
		return fs, err
	}

	// 只有目录文件，才列出所有子选项
	if stat.IsDir() {
		return file.client.ListFiles(file)
	}

	// 本身为文件的话，直接用文件本身
	fs.Files = append(fs.Files, file)
	fs.Total = stat.Size()
	return fs, nil
}
