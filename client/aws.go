package client

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ygidtu/transfer/base/fi"
	"gopkg.in/ini.v1"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// AwsS3Client 连接的配置
type AwsS3Client struct {
	Host   *Proxy
	Proxy  *Proxy
	client *s3.Client
	Bucket string
}

/*
NewS3Client 新建Aws S3 client
@host: 自定义的链接，s3://profile/path/to/target
@proxy: s3支持http和https代理
@bucket: optional, 指定bucket名，默认用bucket list中的第一个
*/
func NewS3Client(host, proxy *Proxy, bucket string) (*AwsS3Client, error) {
	return &AwsS3Client{
		Host: host, Proxy: proxy, Bucket: bucket,
	}, nil
}

// clientType 表明本client的类型
func (_ *AwsS3Client) clientType() TransferClientType {
	return Aws
}

// connect 链接至服务器
func (asc *AwsS3Client) connect() error {

	userHome, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get user  home directory: %v", err)
	}
	credentials := filepath.Join(userHome, ".aws/credentials")
	if _, err := os.Stat(credentials); os.IsNotExist(err) {
		log.Fatalf("aws credential do not exists: %v", err)
	}

	cfgFile, err := ini.Load(credentials)
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	if asc.Host == nil {
		return fmt.Errorf("please set host for aws s3")
	}

	// Load the Shared AWS Configuration (~/.aws/config)
	if asc.Host.Host == "" {
		asc.Host.Host = "default"
	}

	if !cfgFile.HasSection(asc.Host.Host) {
		log.Fatalf("do no have profile for %v", asc.Host.Host)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(asc.Host.Host))

	if cfgFile.Section(asc.Host.Host).HasKey("endpoint_url") {
		cfg.BaseEndpoint = aws.String(cfgFile.Section(asc.Host.Host).Key("endpoint_url").String())
	}

	if asc.Proxy != nil {
		cfg.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(asc.Proxy.URL),
			},
		}
	}

	if err != nil {
		return err
	}

	// Create an Amazon S3 service client
	asc.client = s3.NewFromConfig(cfg)

	if asc.Bucket == "" {
		output, err := asc.client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
		if err != nil {
			return err
		}

		for _, obj := range output.Buckets {
			asc.Bucket = *obj.Name
			break
		}
	}

	return nil
}

// close 关闭服务器的链接
func (asc *AwsS3Client) close() error {
	return nil
}

// listFiles 借助aws的prefix列出所有符合prefix的文件路径
func (asc *AwsS3Client) listFiles(src *File) (FileList, error) {
	files := FileList{Files: []*File{}, Total: 0}

	output, err := asc.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(asc.Bucket), Prefix: aws.String(src.Path),
	})
	if err != nil {
		return files, err
	}

	for _, object := range output.Contents {
		files.Files = append(files.Files, &File{
			Path: *object.Key, Size: *object.Size,
			IsFile: true, client: asc,
		})
		files.Total += *object.Size
	}
	return files, nil
}

// exists 判断某个文件是否存在，根据prefix查文件，并判断key和输入的path是否一致
func (asc *AwsS3Client) exists(path string) bool {
	output, err := asc.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(asc.Bucket), Prefix: aws.String(path),
	})
	if err != nil {
		return false
	}

	for _, object := range output.Contents {
		if strings.TrimRight(path, "/") == strings.TrimRight(*object.Key, "/") {
			return true
		}
	}
	return false
}

// newFile 在aws client无用
func (asc *AwsS3Client) newFile(path string) (*File, error) {
	path = strings.TrimLeft(path, "/")
	if asc.exists(path) {
		output, err := asc.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(asc.Bucket), Prefix: aws.String(path),
		})
		if err != nil {
			return nil, err
		}

		for _, object := range output.Contents {
			if path == *object.Key {
				return &File{Path: path, Size: *object.Size, client: asc, IsFile: true}, nil
			}
		}
	}
	return &File{Path: path, Size: 0, client: asc, IsFile: true}, nil
}

// mkdir 创建文件夹
func (asc *AwsS3Client) mkdir(path string) error {
	if !asc.exists(path) {
		_, err := asc.client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(asc.Bucket), Key: aws.String(path),
		})
		return err
	}
	return nil
}

// mkParent 创建父目录
func (asc *AwsS3Client) mkParent(path string) error {
	return asc.mkdir(filepath.Dir(path))
}

// getMd5 根据文件的大小，有选择的掐头去尾创建MD5
func (asc *AwsS3Client) getMd5(file *File) error {
	log.Debugf("get md5 of %s", file.Path)
	stat, err := asc.stat(file.Path)
	if !os.IsNotExist(err) {
		var data []byte
		r, err := asc.reader(file.Path, 0)
		if err != nil {
			return err
		}
		if stat.Size() < fileSizeLimit {
			data, err = io.ReadAll(r)
			if err := r.Close(); err != nil {
				return err
			}
		} else {
			data = make([]byte, capacity)
			_, err = r.Read(data[:capacity/2])
			if err != nil {
				return err
			}
			if err := r.Close(); err != nil {
				return err
			}
			r, err = asc.reader(file.Path, stat.Size()-capacity/2)
			if err != nil {
				return err
			}
			if err := r.Close(); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		file.Md5 = fmt.Sprintf("%x", md5.Sum(data))
	}

	return err
}

// reader 创建远程文件的ReadCloser
func (asc *AwsS3Client) reader(path string, offset int64) (io.ReadCloser, error) {
	result, err := asc.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(asc.Bucket),
		Key:    aws.String(path),
		Range:  aws.String(fmt.Sprintf("bytes=%d-", offset)), // Replace with the desired offset value
	})

	if err != nil {
		return nil, err
	}

	return result.Body, err
}

// writeAt 在aws模式下无用
func (asc *AwsS3Client) writeAt(_ io.Reader, _ string, _ bool) error {
	return fmt.Errorf("aws do not support writeAt, please use write instead")
}

// write 写出完整文件
func (asc *AwsS3Client) write(reader io.ReadSeeker, path string) error {
	if asc.mkParent(path) == nil {
		_, err := asc.client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(asc.Bucket),
			Key:    aws.String(path),
			Body:   reader,
		})

		return err
	}
	return nil
}

// stat 列出目标文件的信息
func (asc *AwsS3Client) stat(path string) (os.FileInfo, error) {
	if asc.exists(path) {
		output, err := asc.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(asc.Bucket), Prefix: aws.String(path),
		})
		if err != nil {
			return nil, err
		}

		for _, object := range output.Contents {
			if strings.TrimRight(path, "/") == strings.TrimRight(*object.Key, "/") {
				return fi.AwsFileInfo{Object: object}, nil
			}
		}
	}
	return nil, os.ErrNotExist
}
