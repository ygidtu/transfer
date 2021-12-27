package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// GetList used by get function to get all files to download
func GetList() ([]File, error) {
	log.Infof("Get files: %v:%v", host, port)

	target := []File{}
	client := &http.Client{}

	if proxy != nil {
		client.Transport = &http.Transport{
			Proxy:           http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Get(fmt.Sprintf("%v:%v/list", host, port))
	if err != nil {
		return target, fmt.Errorf("failed to get list of files: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return target, fmt.Errorf("failed to read  response from list: %v", err)
	}
	defer resp.Body.Close()

	err = json.Unmarshal(body, &target)
	return target, err
}
