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
	var opt = base.InitOptions()
	if opt.Version {
		base.SugaredLog.Infof("Current version: %s", version)
		base.SugaredLog.Infof("Git Commit Hash: %s", gitHash)
		base.SugaredLog.Infof("UTC Build Time : %s", buildStamp)
		base.SugaredLog.Infof("Golang Version : %s", goVersion)
		os.Exit(0)
	}

	// check the number of threads to use
	if opt.Concurrent < 1 {
		opt.Concurrent = 1
	}

	// check the input source and target
	if opt.Source == "" {
		base.SugaredLog.Fatal("please set source file/directory")
	}

	if opt.Target == "" {
		base.SugaredLog.Fatal("please set target file/directory")
	}

	// init service
	cli, err := client.InitClient(opt)
	if err != nil {
		base.SugaredLog.Fatal(err)
	}

	if opt.Daemon {
		sched := clockwork.NewScheduler()
		sched.Schedule().Every(1).Days().At("12:30").Do(cli.Start)
		sched.Run()
	} else {
		cli.Start()
	}
}
