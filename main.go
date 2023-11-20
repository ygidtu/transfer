package main

import (
	"github.com/whiteshtef/clockwork"
	"github.com/ygidtu/transfer/base"
	"github.com/ygidtu/transfer/client"
	"os"
)

var (
	// version and build info
	buildStamp string
	gitHash    string
	goVersion  string
	version    string
)

func main() {
	var options = base.InitOptions()

	if options.Version {
		base.SugaredLog.Infof("Current version: %s", version)
		base.SugaredLog.Infof("Git Commit Hash: %s", gitHash)
		base.SugaredLog.Infof("UTC Build Time : %s", buildStamp)
		base.SugaredLog.Infof("Golang Version : %s", goVersion)
		os.Exit(0)
	}

	if options.Concurrent < 1 {
		options.Concurrent = 1
	}

	// init service
	cli, err := client.InitClient(options)
	if err != nil {
		base.SugaredLog.Fatal(err)
	}

	if options.Daemon {
		sched := clockwork.NewScheduler()
		sched.Schedule().Every(1).Days().At("12:30").Do(cli.Start)
		sched.Run()
	} else {
		cli.Start()
	}
}
