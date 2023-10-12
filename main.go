package main

import (
	"github.com/schollz/progressbar/v3"
	"github.com/voxelbrain/goptions"
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
	var options = base.Options{}
	goptions.ParseAndFail(&options)

	// ini logger
	base.SetLogger(options.Debug)

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
	if options.Verbs != "" {
		bar := progressbar.New(1)
		cli, err := client.InitClient(&options, bar)
		if err != nil {
			base.SugaredLog.Fatal(err)
		}

		cli.Start()
		err = cli.Close()
		if err != nil {
			base.SugaredLog.Fatal(err)
		}
	} else {
		goptions.PrintHelp()
	}
}
