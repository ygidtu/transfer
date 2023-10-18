package client

import (
	"crypto/md5"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	px "golang.org/x/net/proxy"
	"io"
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
	rsa        string
	Concurrent int
}

func NewSftp(host, proxy *Proxy, rsa string, scp bool, concurrent int) *SftpClient {
	if host.Port == "" {
		host.Port = "22"
	}

	client := &SftpClient{Host: host, SCP: scp, Concurrent: concurrent, rsa: rsa}
	if proxy != nil && proxy.Scheme != "ssh" && proxy.Scheme != "socks5" {
		log.Fatalf("sftp do not support %s proxy", proxy.Scheme)
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
		r, err := os.Open(idRsa)
		if err != nil {
			return nil, fmt.Errorf("failed to open rsa file: %v", err)
		}
		key, err := io.ReadAll(r)
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

// sshClient generate ssh client by id_rsa or password
func sshClient(host *Proxy, rsa string) (*ssh.Client, error) {

	config, err := sshConfig(host.Username, host.Password, rsa)
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

// sshClientConn generate a ssh client connection
func sshClientConn(conn net.Conn, host *Proxy, rsa string) (*ssh.Client, error) {
	config, err := sshConfig(host.Username, host.Password, rsa)
	if err != nil {
		return nil, err
	}

	c, channels, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:%v", host.Host, host.Port), config)
	if err != nil {
		return nil, err
	}

	return ssh.NewClient(c, channels, reqs), nil
}

// Connect to server
func (cliConf *SftpClient) Connect() error {

	if cliConf.Proxy == nil {
		client, err := sshClient(cliConf.Host, cliConf.rsa)

		if err != nil {
			return err
		}
		cliConf.sshClient = client
	} else if cliConf.Proxy.Scheme == "ssh" { // ssh proxy
		// dial to proxy server
		log.Infof("dail through ssh proxy: %s", cliConf.Proxy.Addr())
		proxyClient, err := sshClient(cliConf.Proxy, cliConf.rsa)

		if err != nil {
			return err
		}

		// generate connection through proxy server
		conn, err := proxyClient.Dial("tcp", cliConf.Host.Addr())

		if err != nil {
			return err
		}

		client, err := sshClientConn(conn, cliConf.Host, cliConf.rsa)
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

		client, err := sshClientConn(conn, cliConf.Host, cliConf.rsa)
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

// Close closes the sftp connection
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

// NewFile create a new file on remote server
func (cliConf *SftpClient) NewFile(path string) (*File, error) {

	if _, err := cliConf.sftpClient.Lstat(filepath.Dir(path)); os.IsNotExist(err) {
		err = cliConf.sftpClient.MkdirAll(filepath.Dir(path))
		if err != nil {
			return nil, err
		}
	}

	stat, _ := cliConf.sftpClient.Lstat(path)
	return &File{Path: path, Size: stat.Size(), IsFile: !stat.IsDir(), client: cliConf}, nil
}

// Mkdir as name says
func (cliConf *SftpClient) Mkdir(path string) error {
	if !cliConf.Exists(path) {
		return cliConf.sftpClient.MkdirAll(path)
	}
	return nil
}

// MkParent make parent directory of path
func (cliConf *SftpClient) MkParent(path string) error {
	parent := filepath.Dir(path)
	if !cliConf.Exists(parent) {
		return cliConf.Mkdir(parent)
	}
	return nil
}

func (cliConf *SftpClient) Reader(path string, offset int64) (io.ReadCloser, error) {
	r, err := cliConf.sftpClient.Open(path)
	if err != nil {
		return r, err
	}
	_, err = r.Seek(offset, 0)
	return r, err
}

func (cliConf *SftpClient) WriteAt(reader io.Reader, path string, trunc bool) error {
	writerCode := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if trunc {
		writerCode = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	f, err := cliConf.sftpClient.OpenFile(path, writerCode)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	return err
}

func (cliConf *SftpClient) Stat(path string) (os.FileInfo, error) {
	return cliConf.sftpClient.Lstat(path)
}

func (cliConf *SftpClient) GetMd5(file *File) error {
	if ok := cliConf.Exists(file.Path); ok {

		var data []byte
		if stat, err := cliConf.Stat(file.Path); !os.IsNotExist(err) {
			if stat.Size() < fileSizeLimit {
				reader, err := cliConf.Reader(file.Path, 0)
				if err != nil {
					return err
				}
				data, err = io.ReadAll(reader)
				if err := reader.Close(); err != nil {
					return err
				}
			} else {
				data = make([]byte, capacity)

				reader, err := cliConf.Reader(file.Path, 0)
				if err != nil {
					return err
				}
				_, err = reader.Read(data[:capacity/2])
				if err != nil {
					return err
				}
				if err := reader.Close(); err != nil {
					return err
				}

				reader, err = cliConf.Reader(file.Path, stat.Size()-capacity/2)
				if err != nil {
					return err
				}
				_, err = reader.Read(data[capacity/2:])
				if err != nil {
					return err
				}
				if err := reader.Close(); err != nil {
					return err
				}
			}
		}
		file.Md5 = fmt.Sprintf("%x", md5.Sum(data))
	}
	return nil
}

func (cliConf *SftpClient) ListFiles(file *File) (FileList, error) {
	files := FileList{Files: []*File{}}
	// walk a directory
	if stat, err := cliConf.sftpClient.Stat(file.Path); os.IsNotExist(err) {
		return files, fmt.Errorf("%s not exists: %v", file.Path, err)
	} else if stat.IsDir() {
		w := cliConf.sftpClient.Walk(file.Path)
		for w.Step() {
			if w.Err() != nil {
				log.Warn(w.Err())
			}

			if opt.Skip && w.Path() != "." && w.Path() != "./" {
				if strings.HasPrefix(filepath.Base(w.Path()), ".") {
					continue
				}
			}

			if !w.Stat().IsDir() {
				files.Files = append(
					files.Files,
					&File{Path: w.Path(), Size: w.Stat().Size(), IsFile: true, client: cliConf},
				)
				files.Total += w.Stat().Size()
			}
		}
	} else {
		files.Files = append(
			files.Files,
			&File{Path: file.Path, Size: stat.Size(), IsFile: true, client: cliConf},
		)
		files.Total += stat.Size()
	}
	return files, nil
}