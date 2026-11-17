package rule

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"ctaf-fz/config"

	pkgSFTP "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	// ErrAuth 表示 SFTP 用户名或密码认证失败。
	ErrAuth = errors.New("sftp authentication failed")
	// ErrSSH 表示 SSH/SFTP 连接建立失败。
	ErrSSH = errors.New("ssh connection failed")
	// ErrNotFound 表示远程文件不存在或无法打开。
	ErrNotFound = errors.New("remote file not found")
	// ErrInvalidURL 表示 SFTP 下载地址格式不合法。
	ErrInvalidURL = errors.New("invalid sftp url")
)

// Downloader 负责按配置下载 SFTP 文件到本地目录。
type Downloader struct {
	cfg config.SFTPConfig
}

// DownloadResult 保存一次 SFTP 下载的本地结果。
type DownloadResult struct {
	LocalPath string
	FileName  string
}

// NewDownloader 创建 SFTP 下载器。
func NewDownloader(cfg config.SFTPConfig) *Downloader {
	return &Downloader{cfg: cfg}
}

// Download 下载远程 SFTP 文件，并返回本地保存路径。
func (d *Downloader) Download(ctx context.Context, remotePath string) (DownloadResult, error) {
	remote, err := ParseRemotePath(remotePath)
	if err != nil {
		return DownloadResult{}, ErrInvalidURL
	}

	if err := os.MkdirAll(d.cfg.DownloadDir, d.cfg.DirMode); err != nil {
		return DownloadResult{}, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            remote.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(remote.Password)},
		HostKeyCallback: d.hostKeyCallback(),
		Timeout:         d.cfg.Timeout,
	}

	address := fmt.Sprintf("%s:%d", remote.Host, remote.Port)
	dialer := net.Dialer{Timeout: d.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %v", ErrSSH, err)
	}
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, sshConfig)
	if err != nil {
		return DownloadResult{}, classifySSHError(err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	sftpClient, err := pkgSFTP.NewClient(client)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: %v", ErrSSH, err)
	}
	defer sftpClient.Close()

	remoteFile, err := sftpClient.Open(remote.Path)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%w: 远程文件 %s 不存在！", ErrNotFound, remote.Path)
	}
	defer remoteFile.Close()

	fileName := filepath.Base(remote.Path)
	localPath := filepath.Join(d.cfg.DownloadDir, fileName)
	localFile, err := os.OpenFile(localPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return DownloadResult{}, err
	}
	defer localFile.Close()

	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(localFile, remoteFile)
		copyDone <- err
	}()

	select {
	case <-ctx.Done():
		return DownloadResult{}, ctx.Err()
	case err := <-copyDone:
		if err != nil {
			return DownloadResult{}, err
		}
	}

	return DownloadResult{LocalPath: localPath, FileName: fileName}, nil
}

// hostKeyCallback 按配置选择主机密钥校验策略：跳过校验或校验指纹。
func (d *Downloader) hostKeyCallback() ssh.HostKeyCallback {
	if d.cfg.InsecureSkipHostKeyCheck {
		return ssh.InsecureIgnoreHostKey()
	}
	return d.verifyHostKey
}

// verifyHostKey 校验远程主机密钥指纹是否与配置一致。
func (d *Downloader) verifyHostKey(hostname string, remote net.Addr, key ssh.PublicKey) error {
	fingerprint := ssh.FingerprintSHA256(key)
	if fingerprint != d.cfg.HostKeyFingerprint {
		return fmt.Errorf("host key fingerprint mismatch for %s: got %s", hostname, fingerprint)
	}
	return nil
}

// classifySSHError 将 SSH 握手错误归类为认证失败或连接失败。
func classifySSHError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrSSH, err)
	}
	msg := err.Error()
	if strings.Contains(msg, "unable to authenticate") {
		return fmt.Errorf("%w: 连接失败：用户名或密码错误！", ErrAuth)
	}
	return fmt.Errorf("%w: %v", ErrSSH, err)
}
