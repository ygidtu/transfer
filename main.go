package main

import (
	"github.com/voxelbrain/goptions"
	"go.uber.org/zap"
	"os"
)

var (
	path      string
	host      string
	log       *zap.SugaredLogger
	proxy     *Proxy
	sftpProxy *Proxy
	jsonLog   string
)

func main() {
	var options = options{}
	goptions.ParseAndFail(&options)

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
		initSftp(options.Sftp.Remote, options.Sftp.Download, options.Sftp.Pull, options.Sftp.Scp)
	} else if options.Verbs == "ftp" {
		defaultFtp(&options)
		initFtp(options.Ftp.Host, options.Ftp.Remote, options.Ftp.Pull)
	} else {
		goptions.PrintHelp()
	}

	if options.Remove {
		if _, ok := os.Stat(jsonLog); !os.IsNotExist(ok) {
			if err := os.Remove(jsonLog); err != nil {
				log.Warnf("failed to remove %s: %v", jsonLog, err)
			}
		}
	}
}
