// Copyright (c) Elliot Peele <elliot@bentlogic.net>

package proxy

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"

	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh"
)

var logger = logging.MustGetLogger("sshhttpproxy.proxy")

// SSHProxy is a ssh client that port forwards based on configuration information.
type SSHProxy struct {
	cfg  *Config
	conn *ssh.Client
	ctx  context.Context
	wg   *sync.WaitGroup
	done chan struct{}
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
		cfg:  cfg,
		ctx:  context.Background(),
		wg:   new(sync.WaitGroup),
		done: make(chan struct{}),
	}, nil
}

// WithContext sets the current context value
func (p *SSHProxy) WithContext(ctx context.Context) {
	p.ctx = ctx
}

// Shutdown waits for all connections to stop
func (p *SSHProxy) Shutdown() {
	close(p.done)
	p.wg.Wait()
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
	p.wg.Add(1)
	go func() {
		<-p.done
		if err := conn.Close(); err != nil {
			logger.Errorf("error closing connection: %s", err)
		}
		logger.Infof("ssh connection closed")
		p.wg.Done()
	}()
	p.conn = conn
	return nil
}

// Forward forwards a remote addess to a local port. Set localPort to 0 to generate a random port.
func (p *SSHProxy) Forward(remote, localPort string) (string, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%s", localPort))
	if err != nil {
		return "", err
	}
	p.wg.Add(1)
	go func() {
		for {
			local, err := listener.Accept()
			if err != nil {
				logger.Errorf("error connecting to local port: %s", err)
				return
			}
			go p.handleClient(local, remote)
			select {
			case <-p.done:
				if err := listener.Close(); err != nil {
					logger.Errorf("error shutting down listener: %s", err)
				}
				p.wg.Done()
				return
			}
		}
	}()
	return listener.Addr().String(), nil
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

func (p *SSHProxy) handleClient(local net.Conn, remoteConnect string) {
	logger.Debugf("handle client called")
	remote, err := p.conn.Dial("tcp", remoteConnect)
	if err != nil {
		logger.Errorf("remote dial error: %s", err)
		return
	}
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		_, err := io.Copy(local, remote)
		if err != nil {
			logger.Errorf("error while copying remote -> local: %s", err)
		}
		logger.Debugf("local -> remote done")
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		_, err := io.Copy(remote, local)
		if err != nil {
			logger.Errorf("error while copying local -> remote: %s", err)
		}
		logger.Debugf("remote -> local done")
		wg.Done()
	}()
	p.wg.Add(1)
	go func() {
		logger.Debugf("shutting down %s", remoteConnect)
		wg.Wait()
		if err := local.Close(); err != nil {
			logger.Errorf("error closing local connection: %s", err)
		}
		if err := remote.Close(); err != nil {
			logger.Errorf("error closing remote connection: %s", err)
		}
		p.wg.Done()
	}()
}
