package cmd

import (
	"embed"
	"fmt"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"gitlab.landui.cn/gomod/global"
	"gitlab.landui.cn/gomod/logs"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
	"path"
	"time"
)

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

func Run(client *ssh.Client, shell string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		log := logs.New()
		log.Error("创建session失败", err)
		return "", err
	}
	defer session.Close()
	buf, err := session.CombinedOutput(fmt.Sprintf(shell))
	if err != nil && err.Error() != "Process exited with status 1" {
		log := logs.New()
		log.SetAdditionalInfo("shell", shell).Error("执行shell失败", err)
		return "", err
	}
	logs.New().SetAdditionalInfo("out", string(buf)).SetAdditionalInfo("shell", shell).Info("记录执行命令")
	return string(buf), nil
}

// UploadDirectory 上传文件夹
func UploadDirectory(sftpClient *sftp.Client, localPath string, remotePath string) error {
	localFiles, err := os.ReadDir(localPath)
	if err != nil {
		logs.New().SetAdditionalInfo("path", localPath).Error("读取目录失败", err)
		return err
	}

	for _, backupDir := range localFiles {
		localFilePath := path.Join(localPath, backupDir.Name())
		remoteFilePath := path.Join(remotePath, backupDir.Name())
		if backupDir.IsDir() {
			err := sftpClient.Mkdir(remoteFilePath)
			if err != nil {
				logs.New().SetAdditionalInfo("path", remoteFilePath).Error("远程服务器创建目录失败", err)
				return err
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
		logs.New().SetAdditionalInfo("path", localFilePath).Error("打开文件失败", err)
		return err
	}
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	if err != nil {
		logs.New().SetAdditionalInfo("path", remotePath).Error("远程服务器创建文件失败", err)
		return err

	}
	defer dstFile.Close()

	ff, err := io.ReadAll(srcFile)
	if err != nil {
		logs.New().Error("读取所有文件失败", err)
		return err

	}
	dstFile.Write(ff)
	return nil
}
func uploadFileEmbed(sftpClient *sftp.Client, localFilePath embed.FS, name, remotePath string) error {
	srcFile, err := localFilePath.ReadFile(name)
	//srcFile, err := os.Open(localFilePath)
	if err != nil {
		logs.New().SetAdditionalInfo("path", localFilePath).Error("打开文件失败", err)
		return err
	}
	//defer srcFile.Close()

	var remoteFileName = path.Base(name)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	if err != nil {
		logs.New().SetAdditionalInfo("path", remotePath).Error("远程服务器创建文件失败", err)
		return err

	}
	defer dstFile.Close()

	//ff, err := io.ReadAll(srcFile)
	//if err != nil {
	//	logs.New().Error("读取所有文件失败", err)
	//	return err
	//
	//}
	dstFile.Write(srcFile)
	return nil
}
func CreateSftp(client *ssh.Client) (*sftp.Client, error) {
	sftpClient, err := sftp.NewClient(client)
	return sftpClient, err
}
func SyncSftp(client *ssh.Client, srcDir, destDir string) {
	sftpClient, err := CreateSftp(client)
	defer sftpClient.Close()
	if err != nil {
		logs.New().SetAdditionalInfo("Err", err).Error("创建sftp客户端失败", err)
	}
	fileList, err := global.Shell.ReadDir(srcDir)
	for _, v := range fileList {
		Run(client, "rm -rf "+v.Name())
		err = uploadFileEmbed(sftpClient, global.Shell, srcDir+"/"+v.Name(), destDir)
		if err != nil {
			logs.New().SetAdditionalInfo("file", v.Name()).Error("上传sh文件失败", err)
		}
	}

}
