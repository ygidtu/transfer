package main

import (
	"fmt"
	"github.com/jlaffaye/ftp"
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

func (file *File) GetTarget(source, target *File) *File {
	sourcePath := source.Path
	if source.IsFile {
		sourcePath = filepath.Dir(source.Path)
	}
	path := strings.Replace(file.Path, sourcePath, "", 1)
	path = strings.TrimLeft(path, "/")

	return &File{Path: filepath.Join(target.Path, path), IsLocal: !file.IsLocal, IsFile: file.IsFile}
}

func NewFile(path string) (*File, error) {
	stat, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), IsLocal: true}, nil
	}
	return nil, err
}

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

func ListFilesLocal(file *File) ([]*File, error) {
	var files []*File
	var err error
	var total int64
	if file.IsFile {
		files = append(files, file)
		total += file.Size
	} else {
		log.Infof("List files from %s", file.Path)

		err = filepath.Walk(file.Path, func(p string, info os.FileInfo, err error) error {
			if SkipHidden && info.Name() != "./" && info.Name() != "." {
				if strings.HasPrefix(info.Name(), ".") {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if !info.IsDir() {
				files = append(files, &File{
					Path:    p,
					Size:    info.Size(),
					IsFile:  !info.IsDir(),
					IsLocal: true})

				total += info.Size()
			}
			return nil
		})
	}

	bar = BytesBar(total, "Local")
	return files, err
}

func ListFilesHTTP(file *File) ([]*File, error) {
	var files []*File
	var err error
	var total int64
	if file.IsFile {
		files = append(files, file)
		total += file.Size
	} else {
		log.Infof("List files from %s", file.Path)

		err = filepath.Walk(file.Path, func(p string, info os.FileInfo, err error) error {
			if SkipHidden && info.Name() != "./" && info.Name() != "." {
				if strings.HasPrefix(info.Name(), ".") {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if !info.IsDir() {
				files = append(files, &File{
					Path:    strings.TrimRight(strings.ReplaceAll(p, file.Path, ""), "/"),
					Size:    info.Size(),
					IsFile:  !info.IsDir(),
					IsLocal: true})

				total += info.Size()
			}
			return nil
		})
	}

	return files, err
}

// ListFilesSftp as name says collect files
func ListFilesSftp(cliConf *SftpClient, path string) ([]*File, error) {
	var files []*File
	var total int64
	// walk a directory
	if stat, err := cliConf.sftpClient.Stat(path); os.IsNotExist(err) {
		return files, fmt.Errorf("%s not exists: %v", path, err)
	} else if stat.IsDir() {
		w := cliConf.sftpClient.Walk(path)
		for w.Step() {
			if w.Err() != nil {
				log.Warn(w.Err())
			}

			if SkipHidden && w.Path() != "." && w.Path() != "./" {
				if strings.HasPrefix(filepath.Base(w.Path()), ".") {
					continue
				}
			}

			if !w.Stat().IsDir() {
				files = append(files, &File{Path: w.Path(), Size: w.Stat().Size(), IsFile: true, IsLocal: false})
				total += w.Stat().Size()
			}
		}
	} else {
		files = append(files, &File{Path: path, Size: stat.Size(), IsFile: true, IsLocal: false})
		total += stat.Size()
	}

	bar = BytesBar(total, "Sftp")
	return files, nil
}

// ListFilesFtp as name says collect files
func ListFilesFtp(cliConf *FtpClient, path string) ([]*File, error) {
	var files []*File
	var total int64
	walker := cliConf.Client.Walk(path)
	if walker.Err() != nil {
		return files, walker.Err()
	}

	if walker.Stat().Type == ftp.EntryTypeFile {
		f, err := cliConf.NewFile(path)
		if err != nil {
			return files, err
		}
		files = append(files, f)
		total += f.Size
	} else if walker.Stat().Type == ftp.EntryTypeFolder {
		// walk a directory
		entries, err := cliConf.Client.List(path)
		if err != nil {
			return files, err
		}
		for _, e := range entries {
			if SkipHidden && e.Name != "." && e.Name != "./" && strings.HasPrefix(e.Name, ".") {
				continue
			}
			if e.Type == ftp.EntryTypeFile {
				files = append(files, &File{Path: e.Name, Size: int64(e.Size), IsFile: true, IsLocal: false})
				total += int64(e.Size)
			} else if e.Type == ftp.EntryTypeLink {
				files = append(files, &File{Path: e.Name, Target: e.Target, Size: int64(e.Size), IsFile: true, IsLocal: false})
				total += int64(e.Size)
			}
		}
	}
	bar = BytesBar(total, "Ftp")
	return files, nil
}
