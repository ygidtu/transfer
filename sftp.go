package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// ByteCountDecimal human readable file size
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

//连接的配置
type ClientConfig struct {
	Host       string       //ip
	Port       int          // 端口
	Username   string       //用户名
	Password   string       //密码
	sshClient  *ssh.Client  //ssh client
	sftpClient *sftp.Client //sftp client
}

// generateConfig by id_rsa or password
func generateConfig(password string) ([]ssh.AuthMethod, error) {
	id_rsa := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	methods := []ssh.AuthMethod{}
	if _, err := os.Stat(id_rsa); !os.IsNotExist(err) {
		// var hostKey ssh.PublicKey
		// A public key may be used to authenticate against the remote
		// server by using an unencrypted PEM-encoded private key file.
		//
		// If you have an encrypted private key, the crypto/x509 package
		// can be used to decrypt it.
		key, err := ioutil.ReadFile(id_rsa)
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

	if password != "" {
		methods = append(methods, ssh.Password(password))
	}

	return methods, nil
}

// Connect connect to server
func (cliConf *ClientConfig) Connect() error {
	auth, err := generateConfig(cliConf.Password)
	if err != nil {
		return fmt.Errorf("failed to create client config: %s", err)
	}

	clientConfig := &ssh.ClientConfig{
		User: cliConf.Username,
		Auth: auth,
		// Timeout: 30 * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// connet to ssh
	addr := fmt.Sprintf("%s:%d", cliConf.Host, cliConf.Port)

	conn, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}
	cliConf.sshClient = conn

	client, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("failed to create client: %s", err)
	}
	cliConf.sftpClient = client

	return err
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
func (cliConf *ClientConfig) Upload(srcPath File, dstPath string) error {
	err := cliConf.MkParent(dstPath, true)
	if err != nil {
		return err
	}

	srcFile, _ := os.Open(filepath.Join(path, srcPath.Path)) //本地

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
		}
		if stat.Size() == srcPath.Size {
			log.Infof("Skip: %s", dstPath)
			return nil
		}

		if stat.Size() > srcPath.Size {
			log.Warnf("%s is corrupted", dstPath)
			if err := cliConf.sftpClient.Remove(dstPath); err != nil {
				return fmt.Errorf("failed to remove %s: %s", dstPath, err)
			}
		}
	}

	// start new bar
	bar := pb.Full.Start64(srcPath.Size - seek)
	if seek != 0 {
		if _, err := srcFile.Seek(seek, 0); err != nil {
			return fmt.Errorf("failed to seed %s: %s", srcPath.Path, err)
		}
	}

	// create proxy reader
	barReader := bar.NewProxyReader(srcFile)
	_, err = io.Copy(dstFile, barReader)

	// finish bar
	bar.Finish()

	return err
}

// Download pull file from server
func (cliConf *ClientConfig) Download(srcPath File, dstPath string) error {
	err := cliConf.MkParent(dstPath, false)
	if err != nil {
		return err
	}
	srcFile, _ := cliConf.sftpClient.Open(filepath.Join(path, srcPath.Path))

	dstFile, err := os.OpenFile(dstPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", dstPath, err)
	}
	defer func() {
		_ = srcFile.Close()
		_ = dstFile.Close()
	}()

	var seek int64 = 0
	if stat, err := os.Stat(dstPath); !os.IsNotExist(err) {
		if stat.Size() < srcPath.Size && stat.Size() > 0 {
			log.Infof("Resume %s from %s", dstPath, ByteCountDecimal(stat.Size()))
			seek = stat.Size()
		}
		if stat.Size() == srcPath.Size {
			log.Infof("Skip: %s", dstPath)
			return nil
		}

		if stat.Size() > srcPath.Size {
			log.Warnf("%s is corrupted", dstPath)
			if err := os.Remove(dstPath); err != nil {
				return fmt.Errorf("failed to remove %s: %s", dstPath, err)
			}
		}
	}

	// start new bar
	bar := pb.Full.Start64(srcPath.Size - seek)

	if seek != 0 {
		if _, err := srcFile.Seek(seek, 0); err != nil {
			return fmt.Errorf("failed to seed %s: %s", srcPath.Path, err)
		}
	}
	// create proxy reader
	barReader := bar.NewProxyReader(srcFile)
	_, err = io.Copy(dstFile, barReader)

	// finish bar
	bar.Finish()

	return err
}

// GetFiles as name says collect files
func (cliConf *ClientConfig) GetFiles(path string, pull bool) ([]File, error) {
	files := []File{}
	if pull { // pull from server
		// walk a directory
		w := cliConf.sftpClient.Walk(path)
		for w.Step() {
			if w.Err() != nil {
				log.Warn(w.Err())
			}

			if !w.Stat().IsDir() {
				p := strings.ReplaceAll(w.Path(), path, "")
				p = strings.TrimLeft(p, "/")
				files = append(files, File{Path: p, Size: w.Stat().Size()})
			}
		}

		return files, nil
	} else { // push to server
		return listFiles()
	}
}
