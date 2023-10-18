package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

type HttpClient struct {
	host      *Proxy
	root      *File
	proxy     *Proxy
	transport *http.Transport
	server    bool
}

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
	client.root, err = local.NewFile(host.Path)
	if err != nil {
		return client, err
	}
	return client, nil
}

func (hc *HttpClient) URL() string {
	return strings.TrimRight(hc.host.URL.String(), "/")
}

func (hc *HttpClient) newUrl(url string) (io.ReadCloser, error) {
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

// HttpServer handlers

// ListFilesServer as name says list all files under directory, and wrap into json format to serve
func (hc *HttpClient) ListFilesServer(w http.ResponseWriter, _ *http.Request) {
	var files FileList
	var err error
	local := NewLocal()
	if hc.root != nil {
		files, err = local.ListFiles(hc.root)
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

// GetFilesServer as name says get posted file and save it
func (hc *HttpClient) GetFilesServer(w http.ResponseWriter, req *http.Request) {
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

func (hc *HttpClient) CreateDirServer(w http.ResponseWriter, req *http.Request) {
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

func (hc *HttpClient) GetMd5Server(w http.ResponseWriter, req *http.Request) {
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

func (hc *HttpClient) StatServer(w http.ResponseWriter, req *http.Request) {
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
				statStruct := NewHttpFileInfo(stat)
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

func (hc *HttpClient) StartServer() error {
	log.Info("path: ", hc.root.Path)
	log.Info("host: ", hc.host)

	http.HandleFunc("/list", hc.ListFilesServer)
	http.HandleFunc("/post", hc.GetFilesServer)
	http.HandleFunc("/create", hc.CreateDirServer)
	http.HandleFunc("/md5", hc.GetMd5Server)
	http.HandleFunc("/stat", hc.StatServer)

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

// HttpClient apis

func (hc *HttpClient) Connect() error { return nil }

func (hc *HttpClient) Close() error { return nil }

func (hc *HttpClient) ListFiles(file *File) (FileList, error) {
	if hc.server {
		local := NewLocal()
		return local.ListFiles(file)
	}

	var files FileList
	u, err := hc.newUrl(fmt.Sprintf("%s/list", hc.URL()))

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

func (hc *HttpClient) Exists(path string) bool {
	if hc.server {
		local := NewLocal()
		return local.Exists(path)
	}
	_, err := hc.newUrl(fmt.Sprintf("%s/list?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	return err == nil
}

func (hc *HttpClient) NewFile(path string) (*File, error) {
	if hc.server {
		local := NewLocal()
		f, err := local.NewFile(path)
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
	u, err := hc.newUrl(fmt.Sprintf("%s/list?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
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

func (hc *HttpClient) Mkdir(path string) error {
	_, err := hc.newUrl(fmt.Sprintf("%s/create?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	return err
}

func (hc *HttpClient) MkParent(path string) error {
	return hc.Mkdir(filepath.Dir(strings.TrimLeft(path, hc.root.Path)))
}

func (hc *HttpClient) GetMd5(file *File) error {
	u, err := hc.newUrl(fmt.Sprintf("%s/md5?path=%s", hc.URL(), strings.TrimLeft(file.Path, hc.root.Path)))
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

func (hc *HttpClient) Stat(path string) (os.FileInfo, error) {
	u, err := hc.newUrl(fmt.Sprintf("%s/stat?path=%s", hc.URL(), strings.TrimLeft(path, hc.root.Path)))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(u)
	if err != nil {
		return nil, err
	}
	info := &HttpFileInfo{}
	err = json.Unmarshal(data, info)
	return info, err
}

func (hc *HttpClient) Reader(path string, offset int64) (io.ReadCloser, error) {
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

func (hc *HttpClient) WriteAt(reader io.Reader, path string, trunc bool) error {
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
