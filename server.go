package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
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

/*
##################################
Server
##################################
*/

// ListFiles as name says list all files under directory
func ListFiles(w http.ResponseWriter, _ *http.Request) {

	files, err := listFiles()
	if err != nil {
		log.Error(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(files)
}

// GetFiles as name says get posted file and save it
func GetFiles(w http.ResponseWriter, req *http.Request) {
	var err error
	switch req.Method {
	case "POST":
		{
			var opath string
			mode := "a"
			for k, v := range req.URL.Query() {
				if k == "path" && len(v) > 0 {
					opath = filepath.Join(path, v[0])
				} else if k == "mode" && len(v) > 0 {
					mode = v[0]
				}
			}

			oDir := filepath.Dir(opath)
			if _, err := os.Stat(oDir); os.IsNotExist(err) {
				if err := os.MkdirAll(oDir, os.ModePerm); err != nil {
					e := fmt.Sprintf("failed to create %s: %v", oDir, err)
					log.Error(e)
					break
				}
			}

			var f *os.File
			if mode == "a" {
				f, err = os.OpenFile(opath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
			} else {
				log.Infof("Trunc file %s", opath)
				f, err = os.OpenFile(opath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			}

			if err != nil {
				e := fmt.Sprintf("failed to open %s: %v", opath, err)
				log.Error(e)
				_, _ = io.WriteString(w, e)
				return
			}

			_, err = io.Copy(f, req.Body)
			if err != nil {
				e := fmt.Sprintf("failed to copy %s: %v", opath, err)
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
					opath := filepath.Join(path, v[0])

					if stat, err := os.Stat(opath); !os.IsNotExist(err) {
						_, _ = io.WriteString(w, fmt.Sprintf("%d", stat.Size()))
					} else {
						_, _ = io.WriteString(w, "0")
					}
					break
				}
			}
		}
	}
}

func Clean(w http.ResponseWriter, req *http.Request) {
	err := clean()
	if err != nil {
		io.WriteString(w, fmt.Sprintf("%v", err))
		return
	}
	_, _ = io.WriteString(w, "Success")
}

/*
##################################
Get
##################################
*/

// GetList used by get function to get all files to download
func GetList() ([]*File, error) {
	log.Infof("Get files: %s", host)

	var target []*File
	client := &http.Client{}

	if proxy != nil {
		client.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxy.URL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Get(fmt.Sprintf("%s/list", host))
	if err != nil {
		return target, fmt.Errorf("failed to get list of files: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return target, fmt.Errorf("failed to read  response from list: %v", err)
	}
	err = resp.Body.Close()
	if err != nil {
		return target, err
	}

	err = json.Unmarshal(body, &target)
	return target, err
}

// Get is function that download links
func Get(task *Task) error {
	file := task.Source
	output := filepath.Join(path, file.Path)
	u := fmt.Sprintf("%s/%v", host, url.PathEscape(file.Path))
	if u == "" {
		return fmt.Errorf("empty url")
	}

	// check if output directory or output file exists
	outDir, err := filepath.Abs(filepath.Dir(output))
	if err != nil {
		return fmt.Errorf("download %s failed: %v", u, err)
	}

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create %s: %v", outDir, err)
		}
	}

	req, err := newURL(u)
	if err != nil {
		return err
	}

	if stat, err := os.Stat(output); !os.IsNotExist(err) {
		if stat.Size() == file.Size {
			log.Infof("%s: skip", task.ID)
			return req.Body.Close()
		} else if stat.Size() > file.Size {
			log.Warnf("%v size [%v] > remote [%v], redownload", output, stat.Size(), file.Size)
			_ = os.Remove(output)
		} else {
			log.Infof("Resume from %s", ByteCountDecimal(stat.Size()))
			err = req.seek(stat.Size())
			if err != nil {
				log.Error(err)
			}
		}
	}

	// save to file
	f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", output, err)
	}
	w := bufio.NewWriter(f)

	bar := BytesBar(req.Size, task.ID)
	_, err = io.Copy(io.MultiWriter(w, bar), req.Body)
	if err != nil {
		return fmt.Errorf("failed to copy %s: %v", output, err)
	}

	_ = bar.Finish()
	_ = w.Flush()
	_ = f.Close()

	if stat, err := os.Stat(output); !os.IsNotExist(err) {
		if stat.Size() != file.Size {
			log.Infof("download incomplete: %v != %v", stat.Size(), file.Size)
			if stat.Size() < file.Size {
				return Get(task)
			} else if stat.Size() > file.Size {
				_ = os.Remove(output)
				return Get(task)
			}
		}
	}

	return nil
}

/*
##################################
Post
##################################
*/

// Post is function that post file to server
func Post(task *Task) error {
	file := task.Source
	input := strings.ReplaceAll(file.Path, path, "")
	if input == "" {
		input = filepath.Base(file.Path)
	}
	u := fmt.Sprintf("%v/post?path=%v", host, url.PathEscape(input))

	if u == "" {
		return fmt.Errorf("empty url")
	}
	var start int64
	var total int64
	if stat, err := os.Stat(file.Path); !os.IsNotExist(err) {
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
		if stat.Size() < remoteSize {
			log.Warnf("remote file may broken: local size [%d] < remove size [%d]", stat.Size(), remoteSize)

			u = fmt.Sprintf("%s&mode=t", u)
		} else {
			start = remoteSize
			u = fmt.Sprintf("%s&mode=a", u)
		}
		total = stat.Size()

	} else {
		return fmt.Errorf("%s not exists", input)
	}

	if start == total {
		log.Infof("Skip: %s", input)
		return nil
	} else if start > 0 {
		log.Infof("Resume from: %s", ByteCountDecimal(start))
	}

	// save to file
	f, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", input, err)
	}
	_, _ = f.Seek(start, 0)

	bar := BytesBar(total-start, task.ID)
	reader := progressbar.NewReader(f, bar)
	resp, err := http.Post(u, "", &reader)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %v", string(body), err)
	}
	_ = reader.Close()
	_ = bar.Finish()
	_ = resp.Body.Close()
	_ = f.Close()
	return nil
}

func initServer() {
	log.Info("path: ", path)
	log.Info("host: ", host)

	http.HandleFunc("/list", ListFiles)
	http.HandleFunc("/post", GetFiles)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}

	fs := http.FileServer(http.Dir(path))
	http.Handle("/", http.StripPrefix("/", fs))
	http.HandleFunc("/delete", Clean)

	log.Error(http.ListenAndServe(host, nil))
}

func initTransport(post bool) {
	log.Info("path: ", path)
	log.Info("host: ", host)

	var files []*File
	if post {
		target, err := listFiles()
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, target...)
	} else {
		target, err := GetList()
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, target...)
	}

	for idx, f := range files {
		file := &Task{f, f.Path, fmt.Sprintf("[%d/%d] %s", idx+1, len(files), filepath.Base(f.Path))}
		if f.Path == path {
			file = &Task{f, f.Name(), fmt.Sprintf("[%d/%d] %s", idx+1, len(files), filepath.Base(f.Path))}
		}

		if post {
			if err := Post(file); err != nil {
				log.Warn(err)
			}
		} else {
			if err := Get(file); err != nil {
				log.Warn(err)
			}
		}
	}
}
