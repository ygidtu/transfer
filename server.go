package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v7"
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

/*
##################################
Get
##################################
*/

// GetList used by get function to get all files to download
func GetList() ([]*File, error) {
	log.Infof("Get files: %v:%v", host, port)

	var target []*File
	client := &http.Client{}

	if proxy != nil {
		client.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxy.URL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Get(fmt.Sprintf("%v:%v/list", host, port))
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
func Get(file *File, p *mpb.Progress) error {
	output := filepath.Join(path, file.Path)
	u := fmt.Sprintf("%v:%v/%v", host, port, url.PathEscape(file.Path))
	log.Info("start to download: ", file.Path)
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
			log.Info("download complete")
			return req.Body.Close()
		} else if stat.Size() > file.Size {
			log.Warnf("%v size [%v] > remote [%v], redownload", output, stat.Size(), file.Size)
			_ = os.Remove(output)
		} else {
			log.Infof("Resume from %s", ByteCountDecimal(stat.Size()))
			_ = req.seek(stat.Size())
		}
	}

	// save to file
	f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", output, err)
	}
	w := bufio.NewWriter(f)

	bar := BytesBar(req.Size, filepath.Base(output), p)
	barReader := bar.ProxyReader(req.Body)

	_, err = io.Copy(w, barReader)
	if err != nil {
		return fmt.Errorf("failed to copy %s: %v", output, err)
	}
	_ = barReader.Close()
	if stat, err := os.Stat(output); !os.IsNotExist(err) {
		if stat.Size() != file.Size {
			log.Infof("download incomplete: %v != %v", stat.Size(), file.Size)
			if stat.Size() < file.Size {
				_ = f.Close()
				return Get(file, p)
			} else if stat.Size() > file.Size {
				_ = os.Remove(output)
				return Get(file, p)
			}
		}
	}

	_ = f.Close()
	_ = req.Body.Close()
	return nil
}

/*
##################################
Post
##################################
*/

// Post is function that post file to server
func Post(file *File, p *mpb.Progress) error {
	input := strings.ReplaceAll(file.Path, path, "")
	if input == "" {
		input = filepath.Base(file.Path)
	}
	u := fmt.Sprintf("%v:%v/post?path=%v", host, port, url.PathEscape(input))

	//log.Info("start to post: ", input)
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
	log.Debug(u)

	// save to file
	f, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", input, err)
	}
	_, _ = f.Seek(start, 0)

	bar := BytesBar(total-start, filepath.Base(input), p)

	barReader := bar.ProxyReader(f)
	resp, err := http.Post(u, "", barReader)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %v", string(body), err)
	}
	_ = barReader.Close()
	_ = resp.Body.Close()
	_ = f.Close()
	return nil
}

func initServer() {
	log.Info("path: ", path)
	log.Info("host: ", host)
	log.Info("port: ", port)

	http.HandleFunc("/list", ListFiles)
	http.HandleFunc("/post", GetFiles)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}

	fs := http.FileServer(http.Dir(path))
	http.Handle("/", http.StripPrefix("/", fs))

	log.Error(http.ListenAndServe(fmt.Sprintf("%v:%v", host, port), nil))
}

func initTransport(post bool, threads int) {
	log.Info("path: ", path)
	log.Info("host: ", host)
	log.Info("port: ", port)

	var files []*File
	var wg sync.WaitGroup
	// passed wg will be accounted at p.Wait() call
	p := mpb.New(mpb.WithWaitGroup(&wg), mpb.WithRefreshRate(180*time.Millisecond))
	taskChan := make(chan *Task)

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

	for i := 0; i < threads; i++ {
		wg.Add(1)
		// simulating some work
		go func(post bool, p *mpb.Progress) {
			defer wg.Done()
			for {
				file, ok := <-taskChan

				if !ok {
					break
				}
				if post {
					log.Infof("[%d/%d] start to post: %v", file.ID, len(files), file.Source.Path)
					if err := Post(file.Source, p); err != nil {
						log.Warn(err)
					}
				} else {
					log.Infof("[%d/%d] start to download: %v", file.ID, len(files), file.Source.Path)
					if err := Get(file.Source, p); err != nil {
						log.Warn(err)
					}
				}
			}
		}(post, p)
	}

	for idx, f := range files {
		if f.Path == path {
			taskChan <- &Task{f, f.Name(), idx + 1}
		} else {
			taskChan <- &Task{f, f.Path, idx + 1}
		}
	}

	close(taskChan)
	p.Wait()
}
