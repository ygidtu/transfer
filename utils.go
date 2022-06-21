package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	p := mpb.New(mpb.WithRefreshRate(180 * time.Millisecond))
	if stat, err := os.Stat(path); os.IsNotExist(err) {
		return files, fmt.Errorf("%s not exists: %v", path, err)
	} else if stat.IsDir() {
		log.Infof("List files from %s", path)

		if _, err := os.Stat(filepath.Join(path, jsonLog)); os.IsNotExist(err) {
			var total int64
			bar := p.AddBar(total,
				mpb.PrependDecorators(decor.CountersNoUnit("%d / %d")),
				mpb.AppendDecorators(decor.Percentage()),
			)

			if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if info.Name() == jsonLog {
					return nil
				}
				total += int64(1)
				if !info.IsDir() {
					p = strings.ReplaceAll(p, path, "")
					p = strings.TrimLeft(p, "/")
					files = append(files, &File{Path: p, Size: info.Size()})
				}
				bar.Increment()
				bar.SetTotal(total, false)
				return nil
			}); err != nil {
				return files, err
			}
			bar.SetTotal(total, true)
			content, err := json.MarshalIndent(files, "", "  ")
			if err != nil {
				log.Warnf("failed to save json progress: %v", err)
			}
			_ = ioutil.WriteFile(filepath.Join(path, jsonLog), content, 0644)
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
	ID     int
}

// BytesBar is used to generate progress bar
func BytesBar(size int64, name string, p *mpb.Progress) *mpb.Bar {

	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}
	//p := mpb.New(mpb.WithRefreshRate(180 * time.Millisecond))

	return p.AddBar(size,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
			decor.CountersKibiByte("[% .2f / % .2f]"),
		),
		mpb.AppendDecorators(
			decor.AverageETA(decor.ET_STYLE_GO),
			decor.Name(" | "),
			decor.AverageSpeed(decor.UnitKiB, "% .2f"),
		),
	)
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
