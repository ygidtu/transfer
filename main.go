package main

import (
	"fmt"
	"github.com/voxelbrain/goptions"
	"go.uber.org/zap"
	"net/http"
	"os"
	"path/filepath"
)

var (
	path      string
	host      string
	log       *zap.SugaredLogger
	proxy     *Proxy
	sftpProxy *Proxy
	jsonLog   string
)

func clean() error {
	if _, ok := os.Stat(filepath.Join(path, jsonLog)); !os.IsNotExist(ok) {
		return os.Remove(filepath.Join(path, jsonLog))
	}
	return nil
}

func main() {
	var options = options{}
	goptions.ParseAndFail(&options)

	if options.Version {
		log.Info("Current version: v0.0.3")
		os.Exit(0)
	}

	jsonLog = "transfer_json_filelist.json"

	if options.Verbs == "server" {
		defaultSend(&options)
		initServer()
	} else if options.Verbs == "trans" {
		defaultGet(&options)
		initTransport(options.Trans.Post)
	} else if options.Verbs == "sftp" {
		log.Info("Running on sftp mode")
		defaultSftp(&options)
		initSftp(options.Sftp.Remote, options.Sftp.Path, options.Sftp.Download, options.Sftp.Pull, options.Sftp.Scp)
	} else if options.Verbs == "ftp" {
		defaultFtp(&options)
		initFtp(options.Ftp.Host, options.Ftp.Remote, options.Ftp.Pull)
	} else {
		goptions.PrintHelp()
	}

	if options.Remove {
		if err := clean(); err != nil {
			log.Error(err)
		}

		if options.Verbs == "trans" {
			_, _ = http.Get(fmt.Sprintf("%s/delete", host))
		}
	}
}
