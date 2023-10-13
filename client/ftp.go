package client

import (
	"crypto/md5"
	"fmt"
	"github.com/jlaffaye/ftp"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FtpFileInfo struct {
	file *ftp.Entry
}

func (ffi FtpFileInfo) Name() string {
	return filepath.Base(ffi.file.Name)
}

func (ffi FtpFileInfo) Size() int64 {
	return int64(ffi.file.Size)
}

// Mode return the fake file mode for ftp file
func (ffi FtpFileInfo) Mode() fs.FileMode {
	return fs.ModePerm
}

func (ffi FtpFileInfo) IsDir() bool {
	return ffi.file.Type == ftp.EntryTypeFolder
}

// Sys return the target of symbolic link
func (ffi FtpFileInfo) Sys() any {
	return ffi.file.Target
}

func (ffi FtpFileInfo) ModTime() time.Time {
	return ffi.file.Time
}

// FtpFileReader create an instance of fs.FileReadCloser for ftp file
type FtpFileReader struct {
	client *ftp.ServerConn
	path   string
	offset int64
	whence int
}

func (ffr *FtpFileReader) Read(p []byte) (int, error) {
	resp, err := ffr.client.RetrFrom(ffr.path, uint64(ffr.offset)+uint64(ffr.whence))
	defer resp.Close()
	if err != nil {
		return 0, err
	}
	return resp.Read(p)
}

func (ffr *FtpFileReader) Seek(offset int64, whence int) (int64, error) {
	ffr.offset = offset
	ffr.whence = whence
	return int64(whence) + offset, nil
}

func (ffr *FtpFileReader) Close() error {
	return nil
}

type FtpFileWriter struct {
	client   *ftp.ServerConn
	fullPath string
	writer   io.WriteCloser
	reader   io.ReadCloser
	closed   bool
	mutex    sync.Mutex
	errChan  chan error
	offset   int
}

func (w *FtpFileWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.closed {
		return 0, net.ErrClosed
	}

	if w.writer == nil {
		pipeR, pipeW := io.Pipe()
		w.reader = pipeR
		w.writer = pipeW

		go func() {
			w.errChan <- w.client.StorFrom(w.fullPath, w.reader, uint64(w.offset))
		}()

	}

	return w.writer.Write(p)
}

func (w *FtpFileWriter) Close() error {
	if !w.closed {
		if w.reader != nil {
			if err := w.reader.Close(); err != nil {
				return err
			}
		}

		if w.writer != nil {
			if err := w.writer.Close(); err != nil {
				return err
			}
		}
	}
	w.closed = true
	return nil
}

// FtpClient 连接的配置
type FtpClient struct {
	Host   *Proxy
	Client *ftp.ServerConn
}

func NewFtp(host string) *FtpClient {
	remoteHost, err := CreateProxy(host)
	if err != nil {
		log.Fatalf("wrong format of ssh server [%s]:  %s", host, err)
	}

	if remoteHost.Port == "" {
		remoteHost.Port = "21"
	}

	cfg := &FtpClient{Host: remoteHost}

	if err := cfg.Connect(); err != nil {
		log.Fatal(err)
	}
	return cfg
}

func (fc *FtpClient) Connect() error {
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

func (fc *FtpClient) Close() error {
	return fc.Client.Quit()
}

func (fc *FtpClient) NewFile(path string) (*File, error) {
	stat, err := fc.Client.List(path)

	if len(stat) > 0 && err == nil {
		for _, i := range stat {
			if i.Name == filepath.Base(path) {
				return &File{
					Path: path, Size: int64(i.Size),
					IsFile: i.Type == ftp.EntryTypeFile, IsLocal: false,
				}, nil
			}
		}
	}

	return &File{
		Path: path, Size: 0,
		IsFile: true, IsLocal: false,
	}, os.ErrNotExist
}

func (fc *FtpClient) Exists(path string) bool {
	_, err := fc.Stat(path)
	return !os.IsNotExist(err)
}

func (fc *FtpClient) Stat(path string) (fs.FileInfo, error) {
	stat, err := fc.Client.List(path)
	if len(stat) > 0 && err == nil {
		for _, i := range stat {
			if i.Name == filepath.Base(path) {
				return FtpFileInfo{i}, nil
			}
		}
	}
	return nil, os.ErrNotExist
}

func (fc *FtpClient) Mkdir(path string) error {
	_, err := fc.Stat(path)
	if err != nil {
		return fc.Client.MakeDir(path)
	}
	return nil
}

func (fc *FtpClient) MkParent(path string) error {
	return fc.Mkdir(filepath.Dir(path))
}

func (fc *FtpClient) Reader(path string) (io.ReadSeekCloser, error) {
	if ok := fc.Exists(path); !ok {
		return nil, os.ErrNotExist
	}
	return &FtpFileReader{client: fc.Client, path: path, whence: 0, offset: 0}, nil
}

func (fc *FtpClient) Writer(path string, code int) (io.WriteCloser, error) {
	offset := 0
	if code&os.O_TRUNC != 0 {
		stat, err := fc.Stat(path)
		if err == nil {
			offset = int(stat.Size())
		}
	}
	return &FtpFileWriter{client: fc.Client, fullPath: path, offset: offset}, nil
}

func (fc *FtpClient) GetMd5(file *File) error {
	if stat, err := fc.Stat(file.Path); !os.IsNotExist(err) {
		var data []byte
		r, err := fc.Reader(file.Path)
		if err != nil {
			return err
		}
		if stat.Size() < fileSizeLimit {
			data, err = io.ReadAll(r)
		} else {
			data = make([]byte, capacity)
			_, err = r.Read(data[:capacity/2])
			if err != nil {
				return err
			}
			_, err = r.Seek(stat.Size()-capacity/2, 0)
			if err != nil {
				return err
			}
			_, err = r.Read(data[capacity/2:])
		}
		if err != nil {
			return err
		}

		file.Md5 = fmt.Sprintf("%x", md5.Sum(data))
	}
	return nil
}

func (fc *FtpClient) ListFiles(file *File) (FileList, error) {
	files := FileList{Files: []*File{}}

	walker := fc.Client.Walk(file.Path)
	if walker.Err() != nil {
		return files, walker.Err()
	}

	if walker.Stat().Type == ftp.EntryTypeFile {
		f, err := fc.NewFile(file.Path)
		if err != nil {
			return files, err
		}
		files.Files = append(files.Files, f)
		files.Total += f.Size
	} else if walker.Stat().Type == ftp.EntryTypeFolder {
		// walk a directory
		entries, err := fc.Client.List(file.Path)
		if err != nil {
			return files, err
		}
		for _, e := range entries {
			if opt.Skip && e.Name != "." && e.Name != "./" && strings.HasPrefix(e.Name, ".") {
				continue
			}
			if e.Type == ftp.EntryTypeFile {
				files.Files = append(files.Files, &File{Path: e.Name, Size: int64(e.Size), IsFile: true, IsLocal: false})
				files.Total += int64(e.Size)
			} else if e.Type == ftp.EntryTypeLink {
				files.Files = append(files.Files, &File{Path: e.Name, Target: e.Target, Size: int64(e.Size), IsFile: true, IsLocal: false})
				files.Total += int64(e.Size)
			}
		}
	}

	return files, nil
}
