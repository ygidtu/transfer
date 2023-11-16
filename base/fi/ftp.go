package fi

import (
	"github.com/jlaffaye/ftp"
	"io/fs"
	"path/filepath"
	"time"
)

// FtpFileInfo 实现了os.FileInfo API的Ftp专属FileInfo类
type FtpFileInfo struct {
	File *ftp.Entry
	Root bool
}

// Name 返回文件名
func (ffi FtpFileInfo) Name() string {
	if ffi.Root {
		return "/"
	}
	return filepath.Base(ffi.File.Name)
}

// Size 返回文件大小
func (ffi FtpFileInfo) Size() int64 {
	if ffi.Root {
		return 0
	}
	return int64(ffi.File.Size)
}

// Mode return the fake file mode for ftp file
func (ffi FtpFileInfo) Mode() fs.FileMode {
	return fs.ModePerm
}

// IsDir 返回对象是否为文件夹
func (ffi FtpFileInfo) IsDir() bool {
	if ffi.Root {
		return true
	}
	return ffi.File.Type == ftp.EntryTypeFolder
}

// Sys return the target of symbolic link
func (ffi FtpFileInfo) Sys() any {
	if ffi.Root {
		return ffi.Root
	}
	return ffi.File.Target
}

// ModTime 返回最后修改时间
func (ffi FtpFileInfo) ModTime() time.Time {
	if ffi.Root {
		return time.Now()
	}
	return ffi.File.Time
}
