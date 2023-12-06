package client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// File is used to kept file path and size
type File struct {
	Path   string // 文件路径
	Size   int64  // 文件大小
	IsFile bool   // 是否为文件
	IsLink bool   // 是否为软连接
	ID     string // 文件传输id
	Md5    string // 文件的md5，文件过大时为头尾md5
	client Client // 文件的来源客户端
}

// FileList connect list of files
type FileList struct {
	Files []*File
	Total int64
}

func (file *File) ShortID() string {
	fn := []rune(file.Name())

	maxLen := 20
	if len(fn) > maxLen {
		return fmt.Sprintf("%s...%s", string(fn[:maxLen/2]), string(fn[(len(fn)-maxLen/2):]))
	}

	return string(fn)
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

	// 如果指定的target的文件与待传输文件名字相同，则不再使用join合并通路
	if filepath.Base(target.Path) == filepath.Base(file.Path) {
		dst = &File{Path: target.Path, IsFile: source.IsFile, client: target.client}
	}

	stat, _ := dst.Stat()
	if stat != nil {
		dst.Size = stat.Size()
		dst.IsFile = !stat.IsDir()
	}

	return dst
}

// NewFile create new File object
func NewFile(path string, client Client) (*File, error) {
	return client.newFile(path)
}

// Reader offers file reader
func (file *File) Reader(offset int64) (io.ReadCloser, error) {
	return file.client.reader(file.Path, offset)
}

// ReadSeeker offers ReadSeeker, but only for local file
func (file *File) ReadSeeker() (io.ReadSeekCloser, error) {
	if file.Source() == Local {
		return file.client.(*LocalClient).readSeeker(file.Path)
	}
	return nil, fmt.Errorf("only local client has ReadSeeker")
}

/*
WriteAt write data at specific file position
@reader: the source file reader
@trunc: write file in trunc or append mode
*/
func (file *File) WriteAt(reader io.Reader, trunc bool) error {
	return file.client.writeAt(reader, file.Path, trunc)
}

/*
Write is used to write file from beginning
@reader: the source file reader
*/
func (file *File) Write(reader io.ReadSeeker) error {
	if file.Source() == Aws {
		return file.client.(*AwsS3Client).write(reader, file.Path)
	}
	return fmt.Errorf("only Aws client has Write")
}

// GetMd5 calculate the md5 hash of partial file
func (file *File) GetMd5() error {
	return file.client.getMd5(file)
}

// Exists checks if file exists
func (file *File) Exists() bool {
	return file.client.exists(file.Path)
}

// MkParent used to create parent directory of file
func (file *File) MkParent() error {
	return file.client.mkParent(file.Path)
}

// Stat used to list file info
func (file *File) Stat() (os.FileInfo, error) {
	return file.client.stat(file.Path)
}

// Children call listFiles from client, to get all children files under directory or file itself
func (file *File) Children() (FileList, error) {
	return file.client.listFiles(file)
}

// Source offers the client type
func (file *File) Source() TransferClientType {
	if file.client == nil {
		return NoClient
	}
	return file.client.clientType()
}
