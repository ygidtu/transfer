package fi

import (
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// AwsFileInfo 实现了os.FileInfo API的Aws专属FileInfo类
type AwsFileInfo struct {
	Object types.Object
}

// Name 返回文件名
func (afi AwsFileInfo) Name() string {
	return filepath.Base(*afi.Object.Key)
}

// Size 返回文件大小
func (afi AwsFileInfo) Size() int64 {
	return int64(afi.Object.Size)
}

// Mode return the fake file mode for ftp file
func (afi AwsFileInfo) Mode() fs.FileMode {
	return fs.ModePerm
}

// IsDir 返回对象是否为文件夹，aws由于特殊walk机制，默认全为文件，仅通过后缀判断是否为文件夹
func (afi AwsFileInfo) IsDir() bool {
	return strings.HasSuffix(*afi.Object.Key, "/")
}

// Sys 返回s3.types.Object
func (afi AwsFileInfo) Sys() any {
	return afi.Object
}

// ModTime 返回最后修改时间
func (afi AwsFileInfo) ModTime() time.Time {
	return *afi.Object.LastModified
}
