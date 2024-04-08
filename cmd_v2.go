package cmd

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	ip       string
	port     string
	user     string
	password string
	client   *ssh.Client
}

func MakeNewSSHClient(ip, port, user, password string) *SSHClient {
	config := &ssh.ClientConfig{
		User: user,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 5 * time.Second,
		Auth:    []ssh.AuthMethod{ssh.Password(password)},
	}
	addr := fmt.Sprintf("%s:%s", ip, port)
	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil
	}

	return &SSHClient{
		ip:       ip,
		port:     port,
		user:     user,
		password: password,
		client:   sshClient,
	}
}

func (c *SSHClient) Run(shell string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		c.client.Close()
		// 如果创建session失败，尝试重新初始化sshClient
		client := MakeNewSSHClient(c.ip, c.port, c.user, c.password)
		if client == nil {
			return "", fmt.Errorf("创建session失败后重建SSH Client失败")
		}
		c.client = client.client
		session, err = c.client.NewSession()
		if err != nil {
			return "", fmt.Errorf("重新创建ssh session失败: %s", err.Error())
		}
	}
	defer session.Close()
	buf, err := session.CombinedOutput(shell)
	if err != nil {
		return "", fmt.Errorf("执行shell失败: shell %s  error: %s", shell, err.Error())
	}
	return string(buf), nil
}

func (c *SSHClient) GetIP() string {
	return c.ip
}

func (c *SSHClient) Close() {
	c.client.Close()
}
