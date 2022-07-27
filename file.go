package main

import (
	"fmt"
	"github.com/jlaffaye/ftp"
	"github.com/schollz/progressbar/v3"
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
}

func (file *File) Name() string {
	return filepath.Base(file.Path)
}

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

func (file *File) GetTarget(source, target string) *File {
	path := strings.TrimLeft(file.Path, source)
	path = strings.TrimLeft(path, "/")

	return &File{Path: filepath.Join(target, path), IsLocal: !file.IsLocal, IsFile: file.IsFile}
}

func NewFile(path string) (*File, error) {
	stat, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), IsLocal: true}, nil
	}
	return nil, err
}

func ListFilesLocal(file *File) ([]*File, error) {
	var files []*File
	var err error
	if file.IsFile {
		files = append(files, file)
	} else {
		log.Infof("List files from %s", file.Path)

		bar := progressbar.Default(-1, fmt.Sprintf("Searching files %s", file.Path))
		err = filepath.Walk(file.Path, func(p string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				files = append(files, &File{
					Path:   p,
					Size:   info.Size(),
					IsFile: !info.IsDir(), IsLocal: true})
			}
			_ = bar.Add(1)
			return nil
		})
	}

	return files, err
}

// ListFilesSftp as name says collect files
func ListFilesSftp(cliConf *SftpClient, path string) ([]*File, error) {
	var files []*File

	// walk a directory
	if stat, err := cliConf.sftpClient.Stat(path); os.IsNotExist(err) {
		return files, fmt.Errorf("%s not exists: %v", path, err)
	} else if stat.IsDir() {
		w := cliConf.sftpClient.Walk(path)
		for w.Step() {
			if w.Err() != nil {
				log.Warn(w.Err())
			}

			if !w.Stat().IsDir() {
				files = append(files, &File{Path: w.Path(), Size: w.Stat().Size(), IsFile: true, IsLocal: false})
			}
		}
	} else {
		files = append(files, &File{Path: path, Size: stat.Size(), IsFile: true, IsLocal: false})
	}

	return files, nil
}

// ListFilesFtp as name says collect files
func ListFilesFtp(cliConf *FtpClient, path string) ([]*File, error) {
	var files []*File

	// walk a directory
	entries, err := cliConf.Client.List(path)
	if err != nil {
		return files, err
	}
	for _, e := range entries {
		if e.Type == ftp.EntryTypeFile {
			files = append(files, &File{Path: e.Name, Size: int64(e.Size), IsFile: true, IsLocal: false})
		} else if e.Type == ftp.EntryTypeLink {
			files = append(files, &File{Path: e.Name, Target: e.Target, Size: int64(e.Size), IsFile: true, IsLocal: false})
		}
	}

	return files, nil
}
