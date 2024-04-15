package cmd

import (
	"embed"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"gitlab.landui.cn/gomod/logs"
	"golang.org/x/crypto/ssh"
)

// NewSSHClient 新建并获取ssh链接
func NewSSHClient(ip, port, user, password string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 5 * time.Second,
		Auth:    []ssh.AuthMethod{ssh.Password(password)},
	}
	addr := fmt.Sprintf("%s:%s", ip, port)
	for i := 0; i < 10; i++ {
		client, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		return client, nil
	}

	return nil, errors.Wrap(errors.New("ssh链接失败"), "NewSSHClient")
}

// Run 调用ssh执行shell命令
func Run(client *ssh.Client, shell string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建session失败: %s", err.Error())
	}
	defer session.Close()
	buf, err := session.CombinedOutput(shell)
	if err != nil && err.Error() != "Process exited with status 1" {
		return "", fmt.Errorf("执行shell失败: %s shell: %s", err.Error(), shell)
	}
	return string(buf), nil
}

// CreateSftp 基于ssh链接，创建出sftp链接
func CreateSftp(client *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(client)
}

// SyncSftp sftp同步文件
func SyncSftp(client *ssh.Client, localFilePath embed.FS, srcDir, destDir string) error {
	sftpClient, err := CreateSftp(client)
	if err != nil {
		return fmt.Errorf("创建sftp客户端失败: %s", err.Error())
	}
	defer sftpClient.Close()

	fileList, err := localFilePath.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取文件夹失败: %s srcDir %s", err.Error(), srcDir)
	}
	for _, v := range fileList {
		Run(client, "rm -rf "+v.Name())
		err = uploadFileEmbed(sftpClient, localFilePath, srcDir+"/"+v.Name(), destDir)
		if err != nil {
			return fmt.Errorf("上传sh文件失败: %s", err.Error())
		}
	}
	return nil
}

// UploadDirectory sftp上传文件夹
func UploadDirectory(sftpClient *sftp.Client, localPath string, remotePath string) error {
	localFiles, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("读取目录失败: %s path: %s", err.Error(), localPath)
	}

	for _, backupDir := range localFiles {
		localFilePath := path.Join(localPath, backupDir.Name())
		remoteFilePath := path.Join(remotePath, backupDir.Name())
		if backupDir.IsDir() {
			err := sftpClient.Mkdir(remoteFilePath)
			if err != nil {
				return fmt.Errorf("远程服务器创建目录失败: %s path: %s", err.Error(), remoteFilePath)
			}
			err = UploadDirectory(sftpClient, localFilePath, remoteFilePath)
			if err != nil {
				logs.New().Error("sftp上传文件夹失败", err)
			}
		} else {
			err := uploadFile(sftpClient, path.Join(localPath, backupDir.Name()), remotePath)
			if err != nil {
				logs.New().Error("sftp传文件出现错误", err)
			}
		}
	}

	return nil
}
func uploadFile(sftpClient *sftp.Client, localFilePath string, remotePath string) error {
	srcFile, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %s path: %s", err.Error(), localFilePath)
	}
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	if err != nil {
		return fmt.Errorf("创建文件失败: %s path: %s", err.Error(), remotePath)
	}
	defer dstFile.Close()

	ff, err := io.ReadAll(srcFile)
	if err != nil {
		return fmt.Errorf("读取所有文件失败: %s", err.Error())
	}
	dstFile.Write(ff)
	return nil
}
func uploadFileEmbed(sftpClient *sftp.Client, localFilePath embed.FS, name, remotePath string) error {
	srcFile, err := localFilePath.ReadFile(name)
	if err != nil {
		return fmt.Errorf("打开文件失败: %s path: %v", err.Error(), localFilePath)
	}

	var remoteFileName = path.Base(name)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	if err != nil {
		return fmt.Errorf("创建文件失败: %s path: %s", err.Error(), remotePath)

	}
	defer dstFile.Close()

	dstFile.Write(srcFile)
	return nil
}
