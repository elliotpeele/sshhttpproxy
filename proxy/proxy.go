// Copyright (c) Elliot Peele <elliot@bentlogic.net>

package proxy

import (
	"io"
	"io/ioutil"
	"net"

	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh"
)

var logger = logging.MustGetLogger("sshhttpproxy.proxy")

// SSHProxy is a ssh client that port forwards based on configuration information.
type SSHProxy struct {
	cfg  *Config
	conn *ssh.Client
	// TODO: add context for managing shutdown
}

// Config is used to store configuraiton information for the SSH Proxy
type Config struct {
	PrivateKeyPath string
	RemoteUser     string
	RemoteAddress  string
}

// New creates an instance of an SSHProxy
func New(cfg *Config) (*SSHProxy, error) {
	return &SSHProxy{
		cfg: cfg,
	}, nil
}

// Connect makes the ssh connection to the remote host
func (p *SSHProxy) Connect() error {
	cfg, err := p.makeConfig()
	if err != nil {
		return err
	}
	conn, err := ssh.Dial("tcp", p.cfg.RemoteAddress, cfg)
	if err != nil {
		return err
	}
	// TODO: add connection management goroutine
	// TODO: close connection on shutdown
	p.conn = conn
	return nil
}

// Forward forwards a remote addess to a local port
func (p *SSHProxy) Forward(remote string) (string, error) {
	remoteConn, err := p.conn.Dial("tcp", remote)
	if err != nil {
		return "", err
	}
	// Using :0 will allocate a random available port
	localConn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go func() {
		// TODO: handle routine shutdown
		for {
			client, err := localConn.Accept()
			if err != nil {
				logger.Errorf("error connecting to local port: %s", err)
				return
			}
			p.handleClient(client, remoteConn)
		}
	}()
	return localConn.Addr().String(), nil
}

func (p *SSHProxy) parsePrivateKey() (ssh.Signer, error) {
	buff, err := ioutil.ReadFile(p.cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(buff)
}

func (p *SSHProxy) makeConfig() (*ssh.ClientConfig, error) {
	key, err := p.parsePrivateKey()
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: p.cfg.RemoteUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Always accept key.
			return nil
		},
	}
	return config, nil
}

func (p *SSHProxy) handleClient(local net.Conn, remote net.Conn) {
	defer local.Close()
	chDone := make(chan bool)

	go func() {
		_, err := io.Copy(local, remote)
		if err != nil {
			logger.Errorf("error while copying remote -> local: %s", err)
		}
		chDone <- true
	}()

	go func() {
		_, err := io.Copy(remote, local)
		if err != nil {
			logger.Errorf("error while copying local -> remote: %s", err)
		}
		chDone <- true
	}()

	<-chDone
}
