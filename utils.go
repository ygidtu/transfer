package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// File is used to kept file path and size
type File struct {
	Path string
	Size int64
}

func (file *File) Name() string {
	return filepath.Base(file.Path)
}

func listFiles() ([]*File, error) {
	var files []*File

	if stat, err := os.Stat(path); os.IsNotExist(err) {
		return files, fmt.Errorf("%s not exists: %v", path, err)
	} else if stat.IsDir() {
		log.Infof("List files from %s", path)

		if _, err := os.Stat(filepath.Join(path, jsonLog)); os.IsNotExist(err) {
			bar := progressbar.Default(-1, fmt.Sprintf("Searching files %s", path))
			if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if info.Name() == jsonLog {
					return nil
				}

				if !info.IsDir() {
					p = strings.ReplaceAll(p, path, "")
					p = strings.TrimLeft(p, "/")
					files = append(files, &File{Path: p, Size: info.Size()})
				}
				_ = bar.Add(1)
				return nil
			}); err != nil {
				return files, err
			}
			content, err := json.MarshalIndent(files, "", "  ")
			_ = bar.Finish()
			if err != nil {
				log.Warnf("failed to save json progress: %v", err)
			}
			_ = ioutil.WriteFile(filepath.Join(path, jsonLog), content, os.ModePerm)
		} else {
			log.Infof("Reload file info from: %s", filepath.Join(path, jsonLog))
			content, err := ioutil.ReadFile(filepath.Join(path, jsonLog))
			if err != nil {
				return files, err
			}
			err = json.Unmarshal(content, &files)
			if err != nil {
				return files, err
			}
		}
	} else {
		files = append(files, &File{Path: path, Size: stat.Size()})
	}

	return files, nil
}

// ByteCountDecimal human-readable file size
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// Task is used to handle the source and target path
type Task struct {
	Source *File
	Target string
	ID     string
}

// BytesBar is used to generate progress bar
func BytesBar(size int64, name string) *progressbar.ProgressBar {

	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}

	return progressbar.DefaultBytes(size, name)
}

// Url is used to handle the url issues in transport and sftp download mode
type Url struct {
	URL    string
	Size   int64
	Resume bool
	Name   string
	Body   io.ReadCloser
}

func newURL(url string) (*Url, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	if proxy != nil {

		client.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxy.URL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	res := &Url{URL: url}

	if _, ok := resp.Header["Accept-Ranges"]; ok {
		res.Resume = true
	}

	if size, ok := resp.Header["Content-Length"]; ok {
		s, _ := strconv.ParseInt(size[0], 10, 64)
		res.Size = s
	}

	if name, ok := resp.Header["Content-Disposition"]; ok {
		if len(name) > 0 {
			names := strings.Split(name[0], "filename=")
			res.Name = strings.Trim(names[len(names)-1], "\"")
		}
	} else {
		res.Name = filepath.Base(url)
	}

	res.Body = resp.Body
	return res, nil
}

func (u *Url) seek(pos int64) error {
	if !u.Resume {
		return fmt.Errorf("%s do not support partial request", u.URL)
	}
	req, err := http.NewRequest(http.MethodGet, u.URL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	if proxy != nil {

		client.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxy.URL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", pos))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	u.Body = resp.Body
	u.Size = u.Size - pos
	return nil
}
