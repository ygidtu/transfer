package main

import (
	"context"
	"fmt"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/pkg/sftp"
	"github.com/vbauerster/mpb/v7"
	"golang.org/x/crypto/ssh"
	px "golang.org/x/net/proxy"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ClientConfig 连接的配置
type ClientConfig struct {
	Host       *Proxy
	sshClient  *ssh.Client  //ssh client
	sftpClient *sftp.Client //sftp client
	scpClient  *scp.Client
}

// sshAuth
func sshConfig(username, password string) (*ssh.ClientConfig, error) {
	idRsa := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	var methods []ssh.AuthMethod

	if password != "" {
		log.Infof("Auth through password")
		methods = append(methods, ssh.Password(password))
	} else if _, err := os.Stat(idRsa); !os.IsNotExist(err) {
		log.Infof("Auth through public key")
		// var hostKey ssh.PublicKey
		// A public key may be used to authenticate against the remote
		// server by using an unencrypted PEM-encoded private key file.
		//
		// If you have an encrypted private key, the crypto/x509 package
		// can be used to decrypt it.
		key, err := ioutil.ReadFile(idRsa)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %v", err)
		}

		// Create the Signer for this private key.
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %v", err)
		}

		// Use the PublicKeys method for remote authentication.
		methods = append(methods, ssh.PublicKeys(signer))
	}

	return &ssh.ClientConfig{
		User:            username,
		Auth:            methods,
		Timeout:         60 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

// sshClient generate a ssh client by id_rsa or password
func sshClient(host *Proxy) (*ssh.Client, error) {

	config, err := sshConfig(host.Username, host.Password)
	if err != nil {
		return nil, err
	}

	// connect to ssh
	conn, err := ssh.Dial("tcp", host.Addr(), config)
	log.Infof("connected to %s", host.Addr())
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %s", err)
	}
	return conn, nil
}

func sshClientConn(conn net.Conn, host *Proxy) (*ssh.Client, error) {
	config, err := sshConfig(host.Username, host.Password)
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:%v", host.Host, host.Port), config)

	if err != nil {
		return nil, err
	}

	return ssh.NewClient(c, chans, reqs), nil
}

// Connect to server
func (cliConf *ClientConfig) Connect() error {

	if sftpProxy == nil {
		client, err := sshClient(cliConf.Host)

		if err != nil {
			return err
		}
		cliConf.sshClient = client
	} else if sftpProxy.Scheme == "ssh" { // ssh proxy
		// dial to proxy server
		log.Infof("dail through ssh proxy: %s", sftpProxy.Addr())
		proxyClient, err := sshClient(sftpProxy)

		if err != nil {
			return err
		}

		// generate connection through proxy server
		conn, err := proxyClient.Dial("tcp", cliConf.Host.Addr())

		if err != nil {
			return err
		}

		client, err := sshClientConn(conn, cliConf.Host)
		if err != nil {
			return err
		}

		cliConf.sshClient = client
	} else if sftpProxy.Scheme == "socks5" {
		log.Infof("dail through socks5 proxy: %s", sftpProxy.Addr())
		dialer, err := px.SOCKS5("tcp", sftpProxy.Addr(), nil, px.Direct)
		if err != nil {
			return err
		}

		conn, err := dialer.Dial("tcp", cliConf.Host.Addr())
		if err != nil {
			return err
		}

		client, err := sshClientConn(conn, cliConf.Host)
		if err != nil {
			return err
		}

		cliConf.sshClient = client
	}

	client, err := sftp.NewClient(cliConf.sshClient)
	if err != nil {
		return fmt.Errorf("failed to create client: %s", err)
	}
	cliConf.sftpClient = client

	client_, err := scp.NewClientBySSH(cliConf.sshClient)
	if err != nil {
		return fmt.Errorf("failed to create scp client: %s", err)
	}
	cliConf.scpClient = &client_

	return err
}

func (cliConf *ClientConfig) Close() error {
	if err := cliConf.sftpClient.Close(); err != nil {
		return err
	}
	return cliConf.sshClient.Close()
}

// Exists check whether file or directory exists
func (cliConf *ClientConfig) Exists(path string) bool {
	_, err := cliConf.sftpClient.Lstat(path)
	return !os.IsNotExist(err)
}

// Mkdir as name says
func (cliConf *ClientConfig) Mkdir(path string) error {
	if !cliConf.Exists(path) {
		return cliConf.sftpClient.MkdirAll(path)
	}
	return nil
}

func (cliConf *ClientConfig) MkParent(path string, upload bool) error {
	if upload {
		if stat, ok := os.Stat(path); !os.IsNotExist(ok) && !stat.IsDir() {
			return nil
		}
	}

	parent := filepath.Dir(path)
	if upload {
		if !cliConf.Exists(parent) {
			return cliConf.sftpClient.MkdirAll(parent)
		}
	} else {
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			return os.MkdirAll(parent, 0755)
		}
	}

	return nil
}

// Upload create or resume upload file
func (cliConf *ClientConfig) Upload(srcPath *File, dstPath string, scp bool, prefix string) error {
	err := cliConf.MkParent(dstPath, true)
	if err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %v", dstPath, err)
	}

	// 本地
	srcFile, _ := os.Open(filepath.Join(path, srcPath.Path))
	if path == srcPath.Path {
		srcFile, _ = os.Open(srcPath.Path)
	}

	if scp {
		return cliConf.scpClient.CopyFile(context.Background(), srcFile, dstPath, "0644")
	}

	// append file
	dstFile, err := cliConf.sftpClient.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE) //远程
	if err != nil {
		return fmt.Errorf("failed to open %s on server: %s", dstPath, err)
	}

	defer func() {
		_ = srcFile.Close()
		_ = dstFile.Close()
	}()

	var seek int64 = 0
	if stat, err := cliConf.sftpClient.Stat(dstPath); !os.IsNotExist(err) {
		if stat.Size() < srcPath.Size && stat.Size() > 0 {
			log.Infof("Resume %s from %s", dstPath, ByteCountDecimal(stat.Size()))
			seek = stat.Size()
		} else if stat.Size() == srcPath.Size {
			log.Infof("Skip: %s", dstPath)
			return nil
		} else if stat.Size() > srcPath.Size {
			log.Warnf("%s is corrupted", dstPath)
			if err := cliConf.sftpClient.Remove(dstPath); err != nil {
				return fmt.Errorf("failed to remove %s: %s", dstPath, err)
			}
		}
	}

	bar := BytesBar(srcPath.Size-seek, fmt.Sprintf("%s %s", prefix, filepath.Base(srcFile.Name())))

	if _, err := srcFile.Seek(seek, 0); err != nil {
		return err
	}

	barReader := bar.ProxyReader(srcFile)
	defer barReader.Close()

	// create proxy reader
	_, err = io.Copy(dstFile, barReader)

	//cliConf.scpClient.CopyFile(context.Background(), barReader, dstPath, "0655")
	return err
}

// Download pull file from server
func (cliConf *ClientConfig) Download(srcPath *File, dstPath string, scp bool, prefix string) error {
	err := cliConf.MkParent(dstPath, false)
	if err != nil {
		return err
	}

	srcFile, _ := cliConf.sftpClient.Open(filepath.Join(path, srcPath.Path))
	if path == srcPath.Path {
		srcFile, _ = cliConf.sftpClient.Open(srcPath.Path)
	}

	dstFile, err := os.OpenFile(dstPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", dstPath, err)
	}
	defer func() {
		_ = srcFile.Close()
		_ = dstFile.Close()
	}()

	if scp {
		if path == srcPath.Path {
			return cliConf.scpClient.CopyFromRemote(context.Background(), dstFile, srcPath.Path)
		}
		return cliConf.scpClient.CopyFromRemote(context.Background(), dstFile, filepath.Join(path, srcPath.Path))
	}

	var seek int64 = 0
	if stat, err := os.Stat(dstPath); !os.IsNotExist(err) {
		if stat.Size() < srcPath.Size && stat.Size() > 0 {
			log.Infof("Resume %s from %s", dstPath, ByteCountDecimal(stat.Size()))
			seek = stat.Size()
		} else if stat.Size() == srcPath.Size {
			log.Infof("Skip: %s", dstPath)
			return nil
		} else if stat.Size() > srcPath.Size {
			log.Warnf("%s is corrupted", dstPath)
			if err := os.Remove(dstPath); err != nil {
				return fmt.Errorf("failed to remove %s: %s", dstPath, err)
			}
		}
	}

	bar := BytesBar(srcPath.Size-seek, fmt.Sprintf("%s %s", prefix, filepath.Base(srcFile.Name())))

	if _, err := srcFile.Seek(seek, 0); err != nil {
		return err
	}

	barReader := bar.ProxyReader(srcFile)
	defer barReader.Close()

	// create proxy reader
	_, err = io.Copy(dstFile, barReader)

	return err
}

// GetFiles as name says collect files
func (cliConf *ClientConfig) GetFiles(path string, pull bool) ([]*File, error) {
	var files []*File
	if pull { // pull from server
		// walk a directory
		if stat, err := cliConf.sftpClient.Stat(path); os.IsNotExist(err) {
			return files, fmt.Errorf("%s not exists: %v", path, err)
		} else if stat.IsDir() {
			w := cliConf.sftpClient.Walk(path)
			for w.Step() {
				if w.Err() != nil {
					log.Warn(w.Err())
				}

				if !w.Stat().IsDir() {
					p := strings.ReplaceAll(w.Path(), path, "")
					p = strings.TrimLeft(p, "/")
					files = append(files, &File{Path: p, Size: w.Stat().Size()})
				}
			}
		} else {
			files = append(files, &File{Path: path, Size: stat.Size()})
		}

		return files, nil
	} else { // push to server
		return listFiles()
	}
}

// PushDownload push a download request to server
func (cliConf *ClientConfig) PushDownload(url, dstPath string) error {
	srcPath, err := newURL(url)
	if err != nil {
		return err
	}

	dstPath = filepath.Join(dstPath, srcPath.Name)

	err = cliConf.MkParent(dstPath, true)
	if err != nil {
		return err
	}

	// append file
	dstFile, err := cliConf.sftpClient.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE) //远程
	if err != nil {
		return fmt.Errorf("failed to open %s on server: %s", dstPath, err)
	}

	defer func() {
		_ = dstFile.Close()
	}()

	if stat, err := cliConf.sftpClient.Stat(dstPath); !os.IsNotExist(err) {
		if stat.Size() < srcPath.Size && stat.Size() > 0 {
			log.Infof("Resume %s from %s", dstPath, ByteCountDecimal(stat.Size()))
			srcPath.seek(stat.Size())
		} else if srcPath.Size == stat.Size() {
			log.Infof("Skip: %s", dstPath)
			return nil
		} else if stat.Size() > srcPath.Size {
			log.Warnf("%s is corrupted", dstPath)
			if err := cliConf.sftpClient.Remove(dstPath); err != nil {
				return fmt.Errorf("failed to remove %s: %s", dstPath, err)
			}
		}
	}

	bar := BytesBar(srcPath.Size, filepath.Base(url))

	barReader := bar.ProxyReader(srcPath.Body)
	defer barReader.Close()

	// create proxy reader
	_, err = io.Copy(dstFile, barReader)

	srcPath.Body.Close()
	return err
}

func initSftp(remote string, download, pull, scp bool, threads int) {
	remoteHost, err := CreateProxy(host)
	if err != nil {
		log.Fatalf("wrong format of ssh server [%s]:  %s", host, err)
	}
	var files []*File
	var wg sync.WaitGroup
	// passed wg will be accounted at p.Wait() call
	p := mpb.New(mpb.WithWaitGroup(&wg), mpb.WithRefreshRate(180*time.Millisecond))
	taskChan := make(chan *Task)

	client := &ClientConfig{Host: remoteHost}

	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// check whether target is exists
	log.Infof("Create %s: %v", remote, client.Exists(remote))
	if !client.Exists(remote) {
		if err := client.Mkdir(remote); err != nil {
			log.Fatal(err)
		}
	}

	if download {
		if err := client.PushDownload(path, remote); err != nil {
			log.Fatal(err)
		}
	} else if pull {
		fs, err := client.GetFiles(remote, pull)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	} else {
		fs, err := client.GetFiles(path, pull)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		// simulating some work
		go func(pull, scp bool) {
			defer wg.Done()

			for {
				task, ok := <-taskChan

				if !ok {
					break
				}

				client := &ClientConfig{Host: remoteHost}

				if err := client.Connect(); err != nil {
					log.Fatal(err)
				}

				if pull {
					if err := client.Download(task.Source, task.Target, scp, fmt.Sprintf("[%d/%d]", task.ID, len(files))); err != nil {
						log.Warn(err)
					}
				} else {
					if err := client.Upload(task.Source, task.Target, scp, fmt.Sprintf("[%d/%d]", task.ID, len(files))); err != nil {
						log.Warn(err)
					}
				}

				err = client.Close()
				if err != nil {
					log.Warn(err)
				}
			}
		}(pull, scp)
	}

	for idx, f := range files {
		if pull {
			taskChan <- &Task{f, filepath.Join(path, f.Path), idx + 1}
		} else {
			if f.Path == path {
				taskChan <- &Task{f, filepath.Join(remote, f.Name()), idx + 1}
			} else {
				taskChan <- &Task{f, filepath.Join(remote, f.Path), idx + 1}
			}
		}
	}

	close(taskChan)
	p.Wait()
}
