package main

import (
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	px "golang.org/x/net/proxy"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SftpClient 连接的配置
type SftpClient struct {
	Host       *Proxy
	Proxy      *Proxy
	sshClient  *ssh.Client  //ssh client
	sftpClient *sftp.Client //sftp client
	SCP        bool
	Concurrent int
}

func NewSftp(host, proxy, rsa string, scp bool, concurrent int) *SftpClient {
	remoteHost, err := CreateProxy(host)
	if err != nil {
		log.Fatalf("wrong format of ssh server [%s]:  %s", host, err)
	}
	remoteHost.Path = rsa

	if remoteHost.Port == "" {
		remoteHost.Port = "22"
	}

	client := &SftpClient{Host: remoteHost, SCP: scp, Concurrent: concurrent}
	if proxy != "" {
		p, err := CreateProxy(proxy)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}
		client.Proxy = p
	}

	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}
	log.Infof("connected to %s", client.Host.Addr())
	return client
}

// sshAuth
func sshConfig(username, password, rsa string) (*ssh.ClientConfig, error) {
	idRsa := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	if rsa != "" {
		idRsa = rsa
	}
	var methods []ssh.AuthMethod

	if password != "" {
		log.Debugf("Auth through password")
		methods = append(methods, ssh.Password(password))
	} else if _, err := os.Stat(idRsa); !os.IsNotExist(err) {
		log.Debugf("Auth through public key")
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

	config, err := sshConfig(host.Username, host.Password, host.Path)
	if err != nil {
		return nil, err
	}
	// connect to ssh
	conn, err := ssh.Dial("tcp", host.Addr(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %s", err)
	}
	return conn, nil
}

func sshClientConn(conn net.Conn, host *Proxy) (*ssh.Client, error) {
	config, err := sshConfig(host.Username, host.Password, host.Path)
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
func (cliConf *SftpClient) Connect() error {

	if cliConf.Proxy == nil {
		client, err := sshClient(cliConf.Host)

		if err != nil {
			return err
		}
		cliConf.sshClient = client
	} else if cliConf.Proxy.Scheme == "ssh" { // ssh proxy
		// dial to proxy server
		log.Infof("dail through ssh proxy: %s", cliConf.Proxy.Addr())
		proxyClient, err := sshClient(cliConf.Proxy)

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
	} else if cliConf.Proxy.Scheme == "socks5" {
		log.Infof("dail through socks5 proxy: %s", cliConf.Proxy.Addr())
		dialer, err := px.SOCKS5("tcp", cliConf.Proxy.Addr(), nil, px.Direct)
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

	if cliConf.SCP {
		log.Infof("Running on SCP mode")
		client, err = sftp.NewClient(cliConf.sshClient, sftp.UseConcurrentWrites(true), sftp.UseConcurrentReads(true))
		if err != nil {
			return fmt.Errorf("failed to create client: %s", err)
		}
	}

	cliConf.sftpClient = client

	return err
}

func (cliConf *SftpClient) Close() error {
	if err := cliConf.sftpClient.Close(); err != nil {
		return err
	}
	return cliConf.sshClient.Close()
}

// Exists check whether file or directory exists
func (cliConf *SftpClient) Exists(path string) bool {
	_, err := cliConf.sftpClient.Lstat(path)
	return !os.IsNotExist(err)
}

func (cliConf *SftpClient) NewFile(path string) (*File, error) {
	if _, err := cliConf.sftpClient.Lstat(path); os.IsNotExist(err) {

		err = cliConf.sftpClient.MkdirAll(path)
		if err != nil {
			return nil, err
		}
	}

	stat, _ := cliConf.sftpClient.Lstat(path)
	return &File{
		Path: path, Size: stat.Size(),
		IsFile: !stat.IsDir(), IsLocal: false,
	}, nil
}

// Mkdir as name says
func (cliConf *SftpClient) Mkdir(path string) error {
	if !cliConf.Exists(path) {
		return cliConf.sftpClient.MkdirAll(path)
	}
	return nil
}

func (cliConf *SftpClient) MkParent(path string, local bool) error {
	if local {
		if stat, ok := os.Stat(path); !os.IsNotExist(ok) && !stat.IsDir() {
			return nil
		}
	}

	parent := filepath.Dir(path)
	if local {
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			return os.MkdirAll(parent, 0755)
		}
	} else {
		if !cliConf.Exists(parent) {
			return cliConf.sftpClient.MkdirAll(parent)
		}
	}

	return nil
}

// Put create or resume upload file
func (cliConf *SftpClient) Put(source, target *File) error {
	if source.IsLocal && !target.IsLocal {
		if !cliConf.Exists(filepath.Dir(target.Path)) {
			if err := cliConf.sftpClient.MkdirAll(filepath.Dir(target.Path)); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %v", target.Path, err)
			}
		}

		// 本地
		srcFile, err := os.Open(source.Path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %v", source.Path, err)
		}

		var seek int64 = 0
		if !cliConf.SCP {
			if stat, err := cliConf.sftpClient.Stat(target.Path); !os.IsNotExist(err) {
				if stat.Size() < source.Size && stat.Size() > 0 {
					log.Debugf("Resume %s from %s", target.Path, ByteCountDecimal(stat.Size()))
					seek = stat.Size()
				} else if stat.Size() == source.Size {
					log.Debugf("Skip: %s", target.Path)
					seek = source.Size
				} else if stat.Size() > source.Size {
					log.Warnf("%s is corrupted", target.Path)
					if err := cliConf.sftpClient.Remove(target.Path); err != nil {
						return fmt.Errorf("failed to remove %s: %s", target.Path, err)
					}
				}
			}
		}

		// append file
		dstFile, err := cliConf.sftpClient.OpenFile(target.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE) //远程
		if err != nil {
			return fmt.Errorf("failed to open %s on server: %s", target.Path, err)
		}

		_ = bar.Add64(seek)
		if _, err := srcFile.Seek(seek, 0); err != nil {
			return err
		}

		// create proxy reader
		_, err = io.Copy(io.MultiWriter(bar, dstFile), srcFile)
		_ = srcFile.Close()
		_ = dstFile.Close()

		return err
	}

	return fmt.Errorf("soure file [%v] should be local, target file [%v] should be remote", source, target)
}

// Pull file from server
func (cliConf *SftpClient) Pull(source, target *File) error {
	if !source.IsLocal && target.IsLocal {
		if err := target.CheckParent(); err != nil {
			return err
		}

		srcFile, err := cliConf.sftpClient.Open(source.Path)
		if err != nil {
			return fmt.Errorf("failed to open remove file: %s - %v", target.Path, err)
		}

		dstFile, err := os.OpenFile(target.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to open %s: %s", target.Path, err)
		}

		var seek int64 = 0
		if !cliConf.SCP {
			if stat, err := os.Stat(target.Path); !os.IsNotExist(err) {
				if stat.Size() < source.Size && stat.Size() > 0 {
					log.Infof("Resume %s from %s", target.Path, ByteCountDecimal(stat.Size()))
					seek = stat.Size()
				} else if stat.Size() == source.Size {
					log.Debugf("Skip: %s", target.Path)
					seek = source.Size
				} else if stat.Size() > source.Size {
					log.Warnf("%s is corrupted", target.Path)
					if err := os.Remove(target.Path); err != nil {
						return fmt.Errorf("failed to remove %s: %s", target.Path, err)
					}
				}
			}
		}

		_ = bar.Add64(seek)
		if _, err := srcFile.Seek(seek, 0); err != nil {
			return err
		}
		_, err = io.Copy(io.MultiWriter(bar, dstFile), srcFile)
		_ = srcFile.Close()
		_ = dstFile.Close()

		return err
	}

	return fmt.Errorf("soure file [%v] should be remote, target file [%v] should be local", source, target)
}

// RemoteToRemote transfer files from remote server to another
func RemoteToRemote(sourceClient, targetClient *SftpClient, source, target *File, scp bool) error {
	if source.IsLocal || target.IsLocal {
		return fmt.Errorf("source file [%v] and target file [%v] should both be remote", source.Path, target.Path)
	}
	if !targetClient.Exists(filepath.Dir(target.Path)) {
		if err := targetClient.sftpClient.MkdirAll(filepath.Dir(target.Path)); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %v", target.Path, err)
		}
	}
	srcFile, err := sourceClient.sftpClient.Open(source.Path)
	if err != nil {
		return fmt.Errorf("failed to open remove file: %s - %v", target.Path, err)
	}

	dstFile, err := targetClient.sftpClient.OpenFile(target.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", target.Path, err)
	}

	var seek int64 = 0
	if !scp {
		if stat, err := targetClient.sftpClient.Stat(target.Path); !os.IsNotExist(err) {
			if stat.Size() < source.Size && stat.Size() > 0 {
				log.Debugf("Resume %s from %s", target.Path, ByteCountDecimal(stat.Size()))
				seek = stat.Size()
			} else if stat.Size() == source.Size {
				log.Debugf("Skip: %s", target.Path)
				seek = source.Size
			} else if stat.Size() > source.Size {
				log.Warnf("%s is corrupted", target.Path)
				if err := targetClient.sftpClient.Remove(target.Path); err != nil {
					return fmt.Errorf("failed to remove %s: %s", target.Path, err)
				}
			}
		}
	}

	_ = bar.Add64(seek)
	if _, err := srcFile.Seek(seek, 0); err != nil {
		return fmt.Errorf("failed to seek: %v", err)
	}
	_, err = io.Copy(io.MultiWriter(bar, dstFile), srcFile)
	_ = srcFile.Close()
	_ = dstFile.Close()

	return err
}

func initSftp(opt *options) {
	if !strings.HasPrefix(opt.Sftp.Host, "ssh") {
		opt.Sftp.Host = fmt.Sprintf("ssh://%s", opt.Sftp.Host)
	}

	if opt.Sftp.Path == "" {
		opt.Sftp.Path = "./"
	}

	client := NewSftp(opt.Sftp.Host, opt.Sftp.Proxy, opt.Sftp.IdRsa, opt.Sftp.Scp, opt.Concurrent)

	var targetClient *SftpClient
	if opt.Sftp.Target != "" {
		log.Infof("Transfer data from remote to remote")
		if !strings.HasPrefix(opt.Sftp.Target, "ssh") {
			opt.Sftp.Target = fmt.Sprintf("ssh://%s", opt.Sftp.Target)
		}
		targetClient = NewSftp(opt.Sftp.Target, opt.Sftp.Proxy, opt.Sftp.IdRsa, opt.Sftp.Scp, opt.Concurrent)
	}

	taskChan := make(chan *File)
	for i := 0; i < opt.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				f, ok := <-taskChan

				if !ok {
					break
				}
				bar.Describe(f.ID)

				if targetClient != nil {
					targetFile := f.GetTarget(source, target)
					targetFile.IsLocal = false
					if err := RemoteToRemote(client, targetClient, f, targetFile, opt.Sftp.Scp); err != nil {
						log.Warn(err)
					}
				} else if opt.Sftp.Pull {
					if err := client.Pull(f, f.GetTarget(source, target)); err != nil {
						log.Warn(err)
					}
				} else {
					if err := client.Put(f, f.GetTarget(source, target)); err != nil {
						log.Warn(err)
					}
				}
			}
		}()
	}

	files := make([]*File, 0, 0)
	if targetClient != nil {
		if root, err := client.NewFile(opt.Sftp.Path); err != nil {
			log.Fatal(err)
		} else {
			source = root
		}

		if root, err := targetClient.NewFile(opt.Sftp.Remote); err != nil {
			log.Fatal(err)
		} else {
			target = root
		}

		fs, err := ListFilesSftp(client, opt.Sftp.Path)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	} else if opt.Sftp.Pull {
		if root, err := NewFileCreate(opt.Sftp.Path); err != nil {
			log.Fatal(err)
		} else {
			target = root
		}

		if root, err := client.NewFile(opt.Sftp.Remote); err != nil {
			log.Fatal(err)
		} else {
			source = root
		}

		fs, err := ListFilesSftp(client, opt.Sftp.Remote)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	} else {
		if root, err := NewFile(opt.Sftp.Path); err != nil {
			log.Fatal(err)
		} else {
			source = root
		}

		if root, err := client.NewFile(opt.Sftp.Remote); err != nil {
			log.Fatal(err)
		} else {
			target = root
		}

		fs, err := ListFilesLocal(source)
		if err != nil {
			log.Fatal(err)
		}
		files = append(files, fs...)
	}

	for i, f := range files {
		f.ID = fmt.Sprintf("[%d/%d] %s", i+1, len(files), f.Name())
		taskChan <- f
	}

	close(taskChan)
}
