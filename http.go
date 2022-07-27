package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Url is used to handle the url issues in transport and sftp download mode
type Url struct {
	URL       string
	Size      int64
	Resume    bool
	Name      string
	Body      io.ReadCloser
	Proxy     *Proxy
	Transport *http.Transport
}

func newURL(url string, proxy *Proxy) (*Url, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	res := &Url{URL: url, Proxy: proxy}
	if proxy != nil {
		res.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxy.URL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	client := &http.Client{Transport: res.Transport}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

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

	client := &http.Client{Transport: u.Transport}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", pos))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	u.Body = resp.Body
	u.Size = u.Size - pos
	return nil
}

type HTTPClient struct {
	Host  *Proxy
	Proxy *Proxy
}

func NewHTTPClient(host, proxy string) *HTTPClient {
	remoteHost, err := CreateProxy(host)
	if err != nil {
		log.Fatal(err)
	}

	proxyP, err := CreateProxy(proxy)
	if err != nil {
		log.Fatal(proxyP)
	}
	return &HTTPClient{remoteHost, proxyP}
}

func (hc *HTTPClient) Put(source *File, target *File) error {
	if source.IsLocal && !target.IsLocal {
		u := fmt.Sprintf("%v/post?path=%v", hc.Host.Addr(), url.PathEscape(target.Path))

		if u == "" {
			return fmt.Errorf("empty url")
		}
		var start int64
		var total int64

		resp, err := http.Get(u)
		if err != nil {
			return err
		}

		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		remoteSize, err := strconv.ParseInt(string(content), 10, 64)
		if err != nil {
			return err
		}
		//log.Info(remoteSize)
		if source.Size < remoteSize {
			log.Warnf("remote file may broken: local size [%d] < remove size [%d]", source.Size, remoteSize)

			u = fmt.Sprintf("%s&mode=t", u)
		} else {
			start = remoteSize
			u = fmt.Sprintf("%s&mode=a", u)
		}
		total = source.Size

		if start == total {
			log.Infof("Skip: %s", source.Path)
			return nil
		} else if start > 0 {
			log.Infof("Resume from: %s", ByteCountDecimal(start))
		}

		// save to file
		f, err := os.Open(source.Path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", source.Path, err)
		}
		_, _ = f.Seek(start, 0)

		bar := BytesBar(total-start, source.Name())
		reader := progressbar.NewReader(f, bar)
		resp, err = http.Post(u, "", &reader)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("%s: %v", string(body), err)
		}

		_ = reader.Close()
		_ = resp.Body.Close()
		_ = f.Close()
		_ = bar.Finish()

		return nil
	}
	return fmt.Errorf("soure file [%v] should be local, target file [%v] should be remote", source, target)
}

func (hc *HTTPClient) Pull(source *File, target *File) error {
	if !source.IsLocal && target.IsLocal {
		u := fmt.Sprintf("%s/%v", hc.Host.Addr(), url.PathEscape(source.Path))
		if u == "" {
			return fmt.Errorf("empty url")
		}

		// check if output directory or output file exists
		if err := target.CheckParent(); err != nil {
			return err
		}

		req, err := newURL(u, hc.Proxy)
		if err != nil {
			return err
		}

		if stat, err := os.Stat(target.Path); !os.IsNotExist(err) {
			if stat.Size() == source.Size {
				log.Infof("%s: skip", source.Name())
				return req.Body.Close()
			} else if stat.Size() > source.Size {
				log.Warnf("%v size [%v] > remote [%v], redownload", target.Path, stat.Size(), source.Size)
				_ = os.Remove(target.Path)
			} else {
				log.Infof("Resume from %s", ByteCountDecimal(stat.Size()))
				err = req.seek(stat.Size())
				if err != nil {
					log.Error(err)
				}
			}
		}

		// save to file
		f, err := os.OpenFile(target.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", target.Path, err)
		}
		w := bufio.NewWriter(f)
		bar := BytesBar(req.Size, source.Name())

		_, err = io.Copy(io.MultiWriter(w, bar), req.Body)
		if err != nil {
			return fmt.Errorf("failed to copy %s: %v", target.Path, err)
		}

		if stat, err := os.Stat(target.Path); !os.IsNotExist(err) {
			if stat.Size() != source.Size {
				log.Infof("download incomplete: %v != %v", stat.Size(), source.Size)
			}
		}

		_ = bar.Finish()
		_ = w.Flush()
		_ = f.Close()
		return nil
	}
	return fmt.Errorf("soure file [%v] should be remote, target file [%v] should be local", source, target)
}

// process and set send options
func initHttp(opt *options) {
	if opt.Trans.Host == "" {
		opt.Trans.Host = "127.0.0.1:8000"
	} else {
		if !strings.Contains(opt.Trans.Host, ":") {
			opt.Trans.Host = fmt.Sprintf("%s:8000", opt.Trans.Host)
		}
	}
	if !strings.HasPrefix(opt.Trans.Host, "http") {
		opt.Trans.Host = fmt.Sprintf("http://%s", opt.Trans.Host)
	}

	if opt.Trans.Path == "" {
		opt.Trans.Path = "./"
	} else {
		if abs, err := filepath.Abs(opt.Trans.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Trans.Path, err)
		} else {
			opt.Trans.Path = abs
		}
	}

	client := NewHTTPClient(opt.Trans.Host, opt.Trans.Proxy)
	//

	if opt.Trans.Post {
		root, err := NewFile(opt.Trans.Path)
		if err != nil {
			log.Fatal(err)
		}
		fs, err := ListFilesLocal(root)
		if err != nil {
			log.Fatal(err)
		}

		for _, f := range fs {
			target := f.GetTarget(root.Path, "")
			if err := client.Put(f, target); err != nil {
				log.Warn(err)
			}
		}
	} else {
		//fs := http.Get()
	}
	return
}
