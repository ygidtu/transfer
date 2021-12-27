package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cavaliercoder/grab"
	pb "github.com/cheggaaa/pb/v3"
)

// Download is funciton that download links
func Download(u, output string) error {
	log.Println("start to download: ", u)
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

	if _, err := os.Stat(output); os.IsExist(err) {
		os.Remove(output)
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

	t := time.NewTicker(5000 * time.Millisecond)
	defer t.Stop()

	bar := pb.New64(resp.Size)
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

	if resp.BytesComplete() != resp.Size {
		return fmt.Errorf("dowload incomplete")
	}

	if resp.Err(); err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}

	// check for errors
	return nil
}
