package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v7"
	"github.com/voxelbrain/goptions"
	"go.uber.org/zap"
)

var (
	path      string
	host      string
	port      int
	log       *zap.SugaredLogger
	proxy     *Proxy
	sftpProxy *Proxy
	p         *mpb.Progress
	jsonLog   string
)

// command line parameters
type options struct {
	Help goptions.Help `goptions:"-h, --help, description='Show this help'"`

	goptions.Verbs
	Server struct {
		Path string `goptions:"-i, --path, description='the path contains files'"`
		Host string `goptions:"-h, --host, description='the ip address to listen'"`
		Port int    `goptions:"-p, --port, description='the port to listen'"`
	} `goptions:"server"`
	Trans struct {
		Path    string `goptions:"-i, --path, description='the path to save files'"`
		Host    string `goptions:"-h, --host, description='the target host ip'"`
		Port    int    `goptions:"-p, --port, description='the target port'"`
		Proxy   string `goptions:"-x, --proxy, description='the proxy to use [http or socks5]'"`
		Post    bool   `goptions:"-s, --post, description='the proxy to use [http or socks5]'"`
		Threads int    `goptions:"-t, --threads, description='the threads to use'"`
	} `goptions:"trans"`
	Sftp struct {
		Path     string `goptions:"-l, --local, description='the local path or url'"`
		Host     string `goptions:"-h, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote   string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull     bool   `goptions:"--pull, description='pull files from server'"`
		Proxy    string `goptions:"-x, --proxy, description='the proxy to use [socks5 or ssh://user:passwd@host:port]'"`
		Scp      bool   `goptions:"-s, --scp, description='transfer throught scp instead of sftp'"`
		Download bool   `goptions:"--download, description='download file and save to server'"`
		ProxyD   string `goptions:"--download-proxy, description='the proxy used to download file [socks5 or http]'"`
		Threads  int    `goptions:"-t, --threads, description='the threads to use'"`
	} `goptions:"sftp"`
}

// process and set send options
func defaultSend(opt *options) {
	if opt.Server.Host == "" {
		host = "0.0.0.0"
	} else {
		host = opt.Server.Host
	}

	if opt.Server.Path == "" {
		path = "./"
	} else {
		if abs, err := filepath.Abs(opt.Server.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Server.Path, err)
		} else {
			path = abs
		}
	}

	if opt.Server.Port == 0 {
		port = 8000
	} else {
		port = opt.Server.Port
	}
}

// process and set get options
func defaultGet(opt *options) {
	if opt.Trans.Host == "" {
		host = "127.0.0.1"
	} else {
		host = opt.Trans.Host
	}

	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("http://%v", host)
	}

	if opt.Trans.Path == "" {
		path = "./"
	} else {
		if abs, err := filepath.Abs(opt.Trans.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Trans.Path, err)
		} else {
			path = abs
		}
	}

	if opt.Trans.Port == 0 {
		port = 8000
	} else {
		port = opt.Trans.Port
	}

	if opt.Trans.Threads == 0 {
		opt.Trans.Threads = 1
	}

	if opt.Trans.Proxy != "" {
		p, err := CreateProxy(opt.Trans.Proxy)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}

		proxy = p

		if p.Scheme != "http" && p.Scheme != "socks5" {
			log.Fatalf("http get mode do not support this kind of proxy: %s", p.Scheme)
		}
	}
}

// process and set push options
func defaultSftp(opt *options) {
	host = opt.Sftp.Host
	if !strings.HasPrefix(host, "ssh") {
		host = fmt.Sprintf("ssh://%s", host)
	}

	if opt.Sftp.Path == "" {
		opt.Sftp.Path = "./"
	}

	if opt.Sftp.Pull {
		path = opt.Sftp.Remote
	} else {
		if abs, err := filepath.Abs(opt.Sftp.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Sftp.Path, err)
		} else {
			path = abs
		}
	}

	if opt.Sftp.Proxy != "" {
		p, err := CreateProxy(opt.Sftp.Proxy)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}

		sftpProxy = p

		if p.Scheme != "ssh" && p.Scheme != "socks5" {
			log.Fatalf("sftp mode do not support this kind of proxy: %s", p.Scheme)
		}
	}

	if opt.Sftp.ProxyD != "" {
		p, err := CreateProxy(opt.Sftp.ProxyD)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}

		proxy = p

		if p.Scheme != "http" && p.Scheme != "socks5" {
			log.Fatalf("the download proxy do not support this kind of proxy: %s", p.Scheme)
		}
	}

	if opt.Sftp.Threads == 0 {
		opt.Sftp.Threads = 1
	}
}

type Task struct {
	Source *File
	Target string
	ID     int
}

func main() {
	var options = options{}
	goptions.ParseAndFail(&options)

	jsonLog = "transfer_json_filelist.json"

	var wg sync.WaitGroup
	// passed wg will be accounted at p.Wait() call
	p = mpb.New(mpb.WithWaitGroup(&wg), mpb.WithRefreshRate(180*time.Millisecond))
	taskChan := make(chan *Task)
	var files []*File
	if options.Verbs == "server" {
		defaultSend(&options)

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
	} else if options.Verbs == "trans" {
		defaultGet(&options)

		log.Info("path: ", path)
		log.Info("host: ", host)
		log.Info("port: ", port)

		if options.Trans.Post {
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

		for i := 0; i < options.Trans.Threads; i++ {
			wg.Add(1)
			// simulating some work
			go func(post bool) {
				defer wg.Done()
				for {
					file, ok := <-taskChan

					if !ok {
						break
					}
					if post {
						log.Infof("[%d/%d] start to post: %v", file.ID, len(files), file.Source.Path)
						if err := Post(file.Source); err != nil {
							log.Warn(err)
						}
					} else {
						log.Infof("[%d/%d] start to download: %v", file.ID, len(files), file.Source.Path)
						if err := Get(file.Source); err != nil {
							log.Warn(err)
						}
					}
				}
			}(options.Trans.Post)
		}

	} else if options.Verbs == "sftp" {
		log.Info("Running on sftp mode")
		defaultSftp(&options)

		remote, err := CreateProxy(host)
		if err != nil {
			log.Fatalf("wrong format of ssh server [%s]:  %s", host, err)
		}

		client := &ClientConfig{Host: remote}

		if err := client.Connect(); err != nil {
			log.Fatal(err)
		}
		defer client.Close()

		// check whether target is exists
		if err := client.Mkdir(options.Sftp.Remote); err != nil {
			log.Fatal(err)
		}

		if options.Sftp.Download {
			if err := client.PushDownload(options.Sftp.Path, options.Sftp.Remote); err != nil {
				log.Fatal(err)
			}
		} else if options.Sftp.Pull {
			fs, err := client.GetFiles(options.Sftp.Remote, options.Sftp.Pull)
			if err != nil {
				log.Fatal(err)
			}
			files = append(files, fs...)
		} else {
			fs, err := client.GetFiles(options.Sftp.Path, options.Sftp.Pull)
			if err != nil {
				log.Fatal(err)
			}
			files = append(files, fs...)
		}

		for i := 0; i < options.Sftp.Threads; i++ {
			wg.Add(1)
			// simulating some work
			go func(pull, scp bool) {
				defer wg.Done()

				for {
					task, ok := <-taskChan

					if !ok {
						break
					}

					client := &ClientConfig{Host: remote}

					if err := client.Connect(); err != nil {
						log.Fatal(err)
					}

					if pull {
						if err := client.Download(task.Source, task.Target, scp, fmt.Sprintf("[%d/%d]", task.ID, len(files))); err != nil {
							log.Warn(err)
						}
					} else {
						if err := client.Upload(task.Source, task.Target, scp, fmt.Sprintf("[%d/%d]", task.ID, len(files))); err != nil {
							log.Warn(err)
						}
					}

					err = client.Close()
					if err != nil {
						log.Warn(err)
					}
				}
			}(options.Sftp.Pull, options.Sftp.Scp)
		}
	} else {
		goptions.PrintHelp()
	}

	for idx, f := range files {
		if options.Sftp.Pull {
			taskChan <- &Task{f, filepath.Join(options.Sftp.Path, f.Path), idx + 1}
		} else {
			if f.Path == path {
				taskChan <- &Task{f, filepath.Join(options.Sftp.Remote, f.Name()), idx + 1}
			} else {
				taskChan <- &Task{f, filepath.Join(options.Sftp.Remote, f.Path), idx + 1}
			}
		}
	}

	close(taskChan)
	// wait for passed wg and for all bars to complete and flush
	p.Wait()
}
