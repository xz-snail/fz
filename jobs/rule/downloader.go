package rule

import (
	"fmt"
	"net/url"
	"strconv"
)

// RemoteFile 表示一个可下载的 SFTP 远程文件地址。
type RemoteFile struct {
	Username string
	Password string
	Host     string
	Port     int
	Path     string
}

// ParseRemotePath 解析 sftp://user:pass@host:port/path 格式的远程路径。
func ParseRemotePath(raw string) (RemoteFile, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return RemoteFile{}, err
	}
	if parsed.Scheme != "sftp" || parsed.User == nil || parsed.Hostname() == "" || parsed.Path == "" {
		return RemoteFile{}, fmt.Errorf("invalid sftp url")
	}
	port := 22
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return RemoteFile{}, fmt.Errorf("invalid sftp port: %w", err)
		}
	}
	password, _ := parsed.User.Password()
	return RemoteFile{
		Username: parsed.User.Username(),
		Password: password,
		Host:     parsed.Hostname(),
		Port:     port,
		Path:     parsed.Path,
	}, nil
}
