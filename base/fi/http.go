package fi

import (
	"io/fs"
	"os"
	"time"
)

type HttpFileInfo struct {
	NameS    string    `json:"name"`
	SizeI    int64     `json:"size"`
	ModeI    uint32    `json:"mode"`
	IsDirB   bool      `json:"isDir"`
	ModTimeT time.Time `json:"modTime"`
}

func NewHttpFileInfo(stat os.FileInfo) HttpFileInfo {
	return HttpFileInfo{
		NameS: stat.Name(), SizeI: stat.Size(), ModeI: uint32(stat.Mode()),
		IsDirB: stat.IsDir(), ModTimeT: stat.ModTime(),
	}
}

func (hfi HttpFileInfo) Name() string {
	return hfi.NameS
}

func (hfi HttpFileInfo) Size() int64 {
	return hfi.SizeI
}

// Mode return the fake file mode for http file
func (hfi HttpFileInfo) Mode() fs.FileMode {
	return fs.FileMode(hfi.ModeI)
}

func (hfi HttpFileInfo) IsDir() bool {
	return hfi.IsDirB
}

// Sys return the target of symbolic link
func (hfi HttpFileInfo) Sys() any {
	return ""
}

func (hfi HttpFileInfo) ModTime() time.Time {
	return hfi.ModTimeT
}
