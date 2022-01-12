package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

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
