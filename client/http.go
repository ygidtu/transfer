package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/ygidtu/transfer/base/fi"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HttpClient http客户端的配置，同时涵盖了服务端和客户端两种模式
type HttpClient struct {
	host      *Proxy          // http服务端监听的地址，客户端链接的地址
	root      *File           // 根目录的地址
	proxy     *Proxy          // 客户端支持的http、socks代理
	transport *http.Transport // 客户端的配置文件
	server    bool            // 客户端还是服务端模式
}

/*
NewHTTPClient 新建http配置对象
@host: http服务端监听的地址，客户端链接的地址
@proxy: 客户端支持的http、socks代理
*/
func NewHTTPClient(host, proxy *Proxy) (*HttpClient, error) {
	client := &HttpClient{host: host, proxy: proxy, server: opt.Server != ""}
	if proxy != nil {
		client.transport = &http.Transport{
			Proxy:           http.ProxyURL(client.proxy.URL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	var err error
	local := NewLocal()
	client.root, err = local.newFile(host.Path)
	if err != nil {
		return client, err
	}
	return client, nil
}

// URL 返回http监听的url地址
func (hc *HttpClient) URL() string {
	return strings.TrimRight(hc.host.URL.String(), "/")
}

/*
newResp 获取链接地址的response信息
@url: 目标链接地址
*/
func (hc *HttpClient) newResp(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if req.Response != nil {
		if req.Response.StatusCode == http.StatusNotFound {
			return nil, os.ErrNotExist
		}
		if req.Response.StatusCode == http.StatusNotModified {
			return nil, os.ErrPermission
		}
	}

	client := &http.Client{}
	if hc.transport != nil {
		client.Transport = hc.transport
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

/*
HttpServer handlers
*/

// listFilesServer as name says list all files under directory, and wrap into json format to serve
func (hc *HttpClient) listFilesServer(w http.ResponseWriter, _ *http.Request) {
	var files FileList
	var err error
	local := NewLocal()
	if hc.root != nil {
		files, err = local.listFiles(hc.root)
		if err != nil {
			log.Error(err)
		}

		for _, f := range files.Files {
			f.Path = strings.Replace(f.Path, hc.root.Path, "", 1)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(files)
}

// getFilesServer as name says get posted file and save it
func (hc *HttpClient) getFilesServer(w http.ResponseWriter, req *http.Request) {
	var err error
	switch req.Method {
	case "POST":
		{
			var outputPath string
			mode := "a"
			for k, v := range req.URL.Query() {
				if k == "path" && len(v) > 0 {
					outputPath = filepath.Join(hc.root.Path, v[0])
				} else if k == "mode" && len(v) > 0 {
					mode = v[0]
				}
			}

			oDir := filepath.Dir(outputPath)
			if _, err := os.Stat(oDir); os.IsNotExist(err) {
				if err := os.MkdirAll(oDir, os.ModePerm); err != nil {
					e := fmt.Sprintf("failed to create %s: %v", oDir, err)
					log.Error(e)
					break
				}
			}

			var f *os.File
			if mode == "a" {
				f, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
			} else {
				log.Infof("Trunc file %s", outputPath)
				f, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			}

			if err != nil {
				e := fmt.Sprintf("failed to open %s: %v", outputPath, err)
				log.Error(e)
				_, _ = io.WriteString(w, e)
				return
			}

			_, err = io.Copy(f, req.Body)
			if err != nil {
				e := fmt.Sprintf("failed to copy %s: %v", outputPath, err)
				log.Error(e)
				_, _ = io.WriteString(w, e)
				return
			}
			_ = req.Body.Close()
			_ = f.Close()
			_, _ = io.WriteString(w, "Success")
		}
	case "GET":
		{
			for k, v := range req.URL.Query() {
				if k == "path" && len(v) > 0 {
					path := filepath.Join(hc.root.Path, v[0])
					if stat, err := os.Stat(path); !os.IsNotExist(err) {
						_, _ = io.WriteString(w, fmt.Sprintf("%d", stat.Size()))

					} else {
						_, _ = io.WriteString(w, "0")
						w.WriteHeader(http.StatusNotFound)
					}
					break
				}
			}
		}
	}
}

// createDireServer 在服务器上新建目录
func (hc *HttpClient) createDirServer(w http.ResponseWriter, req *http.Request) {
	for k, v := range req.URL.Query() {
		if k == "path" && len(v) > 0 {
			path := filepath.Join(hc.root.Path, v[0])
			if strings.HasPrefix(v[0], hc.root.Path) {
				path = v[0]
			}

			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				_, _ = io.WriteString(w, err.Error())
				w.WriteHeader(http.StatusNotModified)
			} else {
				_, _ = io.WriteString(w, "done")
			}
			return
		}
	}
}

// getMd5Server 在服务器端计算目标文件的md5并返回
func (hc *HttpClient) getMd5Server(w http.ResponseWriter, req *http.Request) {
	for k, v := range req.URL.Query() {
		if k == "path" && len(v) > 0 {
			path := filepath.Join(hc.root.Path, v[0])
			if strings.HasPrefix(v[0], hc.root.Path) {
				path = v[0]
			}

			if _, err := os.Stat(path); os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotModified)
				_, _ = io.WriteString(w, err.Error())
				return
			} else {
				f, err := NewFile(path, NewLocal())
				if err != nil {
					w.WriteHeader(http.StatusNotModified)
					_, _ = io.WriteString(w, err.Error())
					return
				}
				err = f.GetMd5()
				if err != nil {
					w.WriteHeader(http.StatusNotModified)
					_, _ = io.WriteString(w, err.Error())
					return
				}
				_, _ = io.WriteString(w, f.Md5)
			}
			return
		}
	}
}

// statServer 在服务器端获取文件信息并返回
func (hc *HttpClient) statServer(w http.ResponseWriter, req *http.Request) {
	for k, v := range req.URL.Query() {
		if k == "path" && len(v) > 0 {
			path := filepath.Join(hc.root.Path, v[0])
			if strings.HasPrefix(v[0], hc.root.Path) {
				path = v[0]
			}

			if stat, err := os.Stat(path); err != nil {
				w.WriteHeader(http.StatusNotModified)
				_, _ = io.WriteString(w, err.Error())
			} else {
				statStruct := fi.NewHttpFileInfo(stat)
				if statBytes, err := json.Marshal(statStruct); err == nil {
					_, _ = w.Write(statBytes)
				} else {
					w.WriteHeader(http.StatusNotModified)
					_, _ = io.WriteString(w, err.Error())
				}
			}
			return
		}
	}
}

// startServer 启动服务器端
func (hc *HttpClient) startServer() error {
	log.Info("path: ", hc.root.Path)
	log.Info("host: ", hc.host)

	http.HandleFunc("/list", hc.listFilesServer)
	http.HandleFunc("/post", hc.getFilesServer)
	http.HandleFunc("/create", hc.createDirServer)
	http.HandleFunc("/md5", hc.getMd5Server)
	http.HandleFunc("/stat", hc.statServer)

	if _, ok := os.Stat(opt.Source); os.IsNotExist(ok) {
		if err := os.MkdirAll(opt.Source, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}

	if !hc.root.IsFile {
		http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(hc.root.Path))))
	} else {
		http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(filepath.Dir(hc.root.Path)))))
	}

	return http.ListenAndServe(hc.host.Addr(), nil)
}

/*
客户端相关API实现
*/

// clientType 返回客户端类型
func (hc *HttpClient) clientType() transferClientType {
	if hc.server {
		return HttpS
	}
	return Http
}

// connect 仅实现client接口，该函数在http客户端上无实际意义
func (hc *HttpClient) connect() error { return nil }

// close 仅实现client接口，该函数在http客户端上无实际意义
func (hc *HttpClient) close() error { return nil }

/*
listFiles 向服务器端请求特定目录下所有文件的信息
@file: 目标路径地址
*/
func (hc *HttpClient) listFiles(file *File) (FileList, error) {
	if hc.server {
		local := NewLocal()
		return local.listFiles(file)
	}

	var files FileList
	u, err := hc.newResp(fmt.Sprintf("%s/list", hc.URL()))

	if err != nil {
		return files, err
	}
	content, err := io.ReadAll(u)
	if err != nil {
		return files, err
	}

	err = json.Unmarshal(content, &files)

	for _, f := range files.Files {
		f.client = hc
	}
	return files, err
}

/*
exists 向服务器端请求检查特定文件是否存在
@path: 目标文件路径
*/
func (hc *HttpClient) exists(path string) bool {
	if hc.server {
		local := NewLocal()
		return local.exists(path)
	}
	_, err := hc.newResp(fmt.Sprintf("%s/list?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	return err == nil
}

/*
newFile 若为客户端，则向服务器端请求特定文件对象信息；若为服务器端，则检查本地是否存在该文件并生成文件对象
@path: 目标文件路径
*/
func (hc *HttpClient) newFile(path string) (*File, error) {
	if hc.server {
		local := NewLocal()
		f, err := local.newFile(path)
		if f != nil {
			f.client = hc
		}
		hc.root = f
		return f, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	fSize := int64(0)
	u, err := hc.newResp(fmt.Sprintf("%s/list?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	if err == nil {
		fSizeStr, err := io.ReadAll(u)
		if err == nil {
			fSizeInt, err := strconv.ParseInt(string(fSizeStr), 10, 64)
			if err == nil {
				fSize = fSizeInt
			}
		}
	}

	return &File{
		Path: strings.TrimLeft(path, hc.root.Path), Size: fSize,
		IsFile: !stat.IsDir(), client: hc,
	}, nil
}

/*
mkdir 向服务器端请求在服务器上新建目录
@path: 目标文件路径
*/
func (hc *HttpClient) mkdir(path string) error {
	_, err := hc.newResp(fmt.Sprintf("%s/create?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	return err
}

/*
mkParent 向服务器端请求新建文件的父目录
@path: 目标文件路径
*/
func (hc *HttpClient) mkParent(path string) error {
	return hc.mkdir(filepath.Dir(strings.TrimLeft(path, hc.root.Path)))
}

/*
getMd5 向服务器端请求特定文件的md5信息
@file: 目标文件路径
*/
func (hc *HttpClient) getMd5(file *File) error {
	u, err := hc.newResp(fmt.Sprintf("%s/md5?path=%s", hc.URL(), strings.TrimLeft(file.Path, hc.root.Path)))
	if err != nil {
		return err
	}

	content, err := io.ReadAll(u)
	if err != nil {
		return err
	}
	file.Md5 = string(content)
	return nil
}

/*
stat 向服务器端请求特定文件的文件信息
@path: 目标文件路径
*/
func (hc *HttpClient) stat(path string) (os.FileInfo, error) {
	u, err := hc.newResp(fmt.Sprintf("%s/stat?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(u)
	if err != nil {
		return nil, err
	}
	info := &fi.HttpFileInfo{}
	err = json.Unmarshal(data, info)
	return info, err
}

/*
reader 向服务器端请求特定文件的文件内容
@path: 目标文件路径
@offset: 读取文件的起始位置
*/
func (hc *HttpClient) reader(path string, offset int64) (io.ReadCloser, error) {
	client := &http.Client{}
	if hc.transport != nil {
		client.Transport = hc.transport
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

/*
writeAt 向服务器端请求特定文件的文件内容
@reader: 源文件的reader
@path: 目标文件路径
@trunc: 写入文件的模式trunc或者append
*/
func (hc *HttpClient) writeAt(reader io.Reader, path string, trunc bool) error {
	mode := "a"
	if trunc {
		mode = "t"
	}

	u := fmt.Sprintf("%v/post?path=%v&mode=%s", hc.URL(), url.PathEscape(path), mode)
	req, err := http.NewRequest(http.MethodPost, u, reader)
	if err != nil {
		return err
	}

	client := &http.Client{}
	if hc.transport != nil {
		client.Transport = hc.transport
	}
	_, err = client.Do(req)
	return err
}
