package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	pb "github.com/cheggaaa/pb/v3"
)

// Download is funciton that download links
func Download(file File) error {
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
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %v", outDir, err)
		}
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	var startByte int64 = 0
	if stat, err := os.Stat(output); !os.IsNotExist(err) {
		if stat.Size() == file.Size {
			log.Info("download complete")
			return nil
		} else if stat.Size() > file.Size {
			log.Warnf("%v size [%v] > remote [%v], redownload", output, stat.Size(), file.Size)
			os.Remove(output)
		} else {
			startByte = stat.Size()
		}
	}

	// set request range
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))

	// create client
	client := &http.Client{}
	if transport != nil {
		client.Transport = transport
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// save to file
	f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)

	// set progress bar
	bar := pb.New64(file.Size - startByte)
	bar.Set(pb.Bytes, true)

	bar.Start()
	barReader := bar.NewProxyReader(resp.Body)
	_, err = io.Copy(w, barReader)
	if err != nil {
		return err
	}
	bar.Finish()
	w.Flush()
	f.Close()

	if stat, err := os.Stat(output); !os.IsNotExist(err) {
		if stat.Size() != file.Size {
			log.Infof("download incomplete: %v != %v", stat.Size(), file.Size)
			if stat.Size() < file.Size {
				f.Close()
				return Download(file)
			} else if stat.Size() > file.Size {
				os.Remove(output)
				return Download(file)
			}
		}
	}

	return nil
}
