package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cavaliercoder/grab"
	pb "github.com/cheggaaa/pb/v3"
)

// Download is funciton that download links
func Download(u, output string) error {
	log.Info("start to download: ", u)
	if u == "" {
		return fmt.Errorf("empty url")
	}

	// check if output directory or output file exists
	outDir, err := filepath.Abs(filepath.Dir(output))
	if err != nil {
		return fmt.Errorf("download %s failed: %v", u, err)
	}

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %v", outDir, err)
		}
	}

	req, err := grab.NewRequest(output, u)

	if err != nil {
		return err
	}

	// setup proxy
	c := grab.NewClient()
	if transport != nil {
		// create http client
		c.HTTPClient.Transport = transport
	}

	resp := c.Do(req)
	bar := pb.New64(resp.Size)

	if stat, err := os.Stat(output); os.IsExist(err) {
		if stat.Size() == resp.Size {
			log.Info("download complete")
			return nil
		} else if stat.Size() > resp.Size {
			log.Warnf("%v size [%v] > remote [%v], redownload", output, stat.Size(), resp.Size)
			os.Remove(output)
		} else {
			bar = pb.New64(resp.Size - stat.Size())
		}
	}

	t := time.NewTicker(5000 * time.Millisecond)
	defer t.Stop()

	bar.Set(pb.Bytes, true)
	bar.Start()
	defer bar.Finish()

Loop:
	for {
		select {
		case <-t.C:
			bar.SetCurrent(resp.BytesComplete())
		case <-resp.Done:
			// download is complete
			break Loop
		}
	}

	if stat, err := os.Stat(output); !os.IsNotExist(err) {
		if stat.Size() < resp.Size {
			log.Warn("dowload incomplete")
			os.Remove(output)
			return Download(u, output)
		}
	} else if os.IsNotExist(err) {
		log.Warn("dowload incomplete")
		return Download(u, output)
	}

	if resp.Err(); err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}

	// check for errors
	return nil
}
