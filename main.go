package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

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
)

// comand line parameters
type options struct {
	Help goptions.Help `goptions:"-h, --help, description='Show this help'"`

	goptions.Verbs
	Send struct {
		Path string `goptions:"-i, --path, description='the path contains files'"`
		Host string `goptions:"-h, --host, description='the ip address to listern'"`
		Port int    `goptions:"-p, --port, description='the port to listern'"`
	} `goptions:"send"`
	Get struct {
		Path  string `goptions:"-i, --path, description='the path to save files'"`
		Host  string `goptions:"-h, --host, description='the target host ip'"`
		Port  int    `goptions:"-p, --port, description='the target port'"`
		Proxy string `goptions:"-x, --proxy, description='the proxy to use [http or socks5]'"`
	} `goptions:"get"`
	Sftp struct {
		Path     string `goptions:"-l, --local, description='the local path or url'"`
		Host     string `goptions:"-h, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote   string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull     bool   `goptions:"--pull, description='pull files from server'"`
		Proxy    string `goptions:"-x, --proxy, description='the proxy to use [socks5 or ssh://user:passwd@host:port]'"`
		Cover    bool   `goptions:"-c, --cover, description='cover old files if exists'"`
		Download bool   `goptions:"--download, description='download file and save to server'"`
		ProxyD   string `goptions:"--download-proxy, description='the proxy used to download file [socks5 or http]'"`
	} `goptions:"sftp"`
}

// process and set send options
func defaultSend(opt options) {
	if opt.Send.Host == "" {
		host = "0.0.0.0"
	} else {
		host = opt.Send.Host
	}

	if opt.Send.Path == "" {
		path = "./"
	} else {
		path = opt.Send.Path
	}

	if opt.Send.Port == 0 {
		port = 8000
	} else {
		port = opt.Send.Port
	}
}

// process and set get options
func defaultGet(opt options) {
	if opt.Get.Host == "" {
		host = "127.0.0.1"
	} else {
		host = opt.Get.Host
	}

	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("http://%v", host)
	}

	if opt.Get.Path == "" {
		path = "./"
	} else {
		path = opt.Get.Path
	}

	if opt.Get.Port == 0 {
		port = 8000
	} else {
		port = opt.Get.Port
	}

	if opt.Get.Proxy != "" {
		p, err := CreateProxy(opt.Get.Proxy)
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
func defaultSftp(opt options) {
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
		path = opt.Sftp.Path
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
}

// File is used to kept file path and size
type File struct {
	Path string
	Size int64
}

// ByteCountDecimal human readable file size
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

func main() {
	var options = options{}
	goptions.ParseAndFail(&options)

	if options.Verbs == "send" {
		defaultSend(options)

		log.Info("path: ", path)
		log.Info("host: ", host)
		log.Info("port: ", port)

		http.HandleFunc("/list", ListFiles)

		fs := http.FileServer(http.Dir(path))
		http.Handle("/", http.StripPrefix("/", fs))

		log.Error(http.ListenAndServe(fmt.Sprintf("%v:%v", host, port), nil))
	} else if options.Verbs == "get" {
		defaultGet(options)

		log.Info("path: ", path)
		log.Info("host: ", host)
		log.Info("port: ", port)

		targets, err := GetList()
		if err != nil {
			log.Error(err)
		}

		for idx, file := range targets {
			log.Infof("[%d/%d] start to download: %v", idx+1, len(targets), file.Path)
			if err := Download(file); err != nil {
				log.Warn(err)
			}
		}
	} else if options.Verbs == "sftp" {
		defaultSftp(options)

		remote, err := CreateProxy(host)
		if err != nil {
			log.Fatalf("wrong format of ssh server [%s]:  %s", host, err)
		}

		client := &ClientConfig{Host: remote}

		if err := client.Connect(); err != nil {
			log.Fatal(err)
		}

		// check whether target is exists
		if err := client.Mkdir(options.Sftp.Remote); err != nil {
			log.Fatal(err)
		}

		var files []File

		if options.Sftp.Download {
			if err := client.PushDownload(options.Sftp.Path, options.Sftp.Remote, options.Sftp.Cover); err != nil {
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

		for idx, f := range files {
			if options.Sftp.Pull {
				log.Infof("[%d/%d] %s <- %s", idx+1, len(files), f.Path, filepath.Join(options.Sftp.Path, f.Path))
				if err := client.Download(f, filepath.Join(options.Sftp.Path, f.Path), options.Sftp.Cover); err != nil {
					log.Warn(err)
				}
			} else {
				log.Infof("[%d/%d] %s -> %s", idx+1, len(files), f.Path, filepath.Join(options.Sftp.Remote, f.Path))
				if err := client.Upload(f, filepath.Join(options.Sftp.Remote, f.Path), options.Sftp.Cover); err != nil {
					log.Warn(err)
				}
			}
		}

		defer client.sftpClient.Close()
		defer client.sshClient.Close()
	}
}
