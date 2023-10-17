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
	root bool
}

func (ffi FtpFileInfo) Name() string {
	if ffi.root {
		return "/"
	}
	return filepath.Base(ffi.file.Name)
}

func (ffi FtpFileInfo) Size() int64 {
	if ffi.root {
		return 0
	}
	return int64(ffi.file.Size)
}

// Mode return the fake file mode for ftp file
func (ffi FtpFileInfo) Mode() fs.FileMode {
	return fs.ModePerm
}

func (ffi FtpFileInfo) IsDir() bool {
	if ffi.root {
		return true
	}
	return ffi.file.Type == ftp.EntryTypeFolder
}

// Sys return the target of symbolic link
func (ffi FtpFileInfo) Sys() any {
	if ffi.root {
		return ffi.root
	}
	return ffi.file.Target
}

func (ffi FtpFileInfo) ModTime() time.Time {
	if ffi.root {
		return time.Now()
	}
	return ffi.file.Time
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

func NewFtp(host *Proxy) *FtpClient {
	if host.Port == "" {
		host.Port = "21"
	}

	cfg := &FtpClient{Host: host}

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

func (fc *FtpClient) Exists(path string) bool {
	_, err := fc.Stat(path)
	return !os.IsNotExist(err)
}

func (fc *FtpClient) Stat(path string) (fs.FileInfo, error) {
	stat, err := fc.Client.List(path)
	if len(stat) > 0 && err == nil {
		for _, i := range stat {
			if i.Name == filepath.Base(path) {
				return FtpFileInfo{file: i}, nil
			}
		}
	}
	if path == "/" {
		return FtpFileInfo{root: true}, nil
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

func (fc *FtpClient) Reader(path string, offset int64) (io.ReadCloser, error) {
	if ok := fc.Exists(path); !ok {
		return nil, os.ErrNotExist
	}
	log.Infof("get path: %s %v", path, offset)
	return fc.Client.RetrFrom(path, uint64(offset))
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
		r, err := fc.Reader(file.Path, 0)
		defer r.Close()
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

			r, err = fc.Reader(file.Path, stat.Size()-capacity/2)
			if err != nil {
				return err
			}
			defer r.Close()
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

	stat, err := fc.Stat(file.Path)
	if err != nil {
		return files, err
	}

	if !stat.IsDir() {
		f, err := fc.NewFile(file.Path)
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
					&File{Path: e.Name, Target: e.Target, Size: int64(e.Size), IsFile: true, client: fc},
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
