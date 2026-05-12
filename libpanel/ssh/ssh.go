// Package ssh wraps golang.org/x/crypto/ssh with conveniences shared
// across 1Panel-family products: SudoHandleCmd auto-detection, Runf
// for printf-style remote commands, CpFileWithCheck with md5 retry,
// streaming stdout/stderr, and optional outbound proxy (HTTP/HTTPS/
// SOCKS5) resolved via a host-app-provided ProxyResolver.
//
// Apps register optional Logger + ProxyResolver via SetLogger /
// SetProxyResolver during init. Without a ProxyResolver, direct dial
// is used regardless of UseProxy in ConnInfo.
package ssh

import (
	"errors"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// ConnInfo carries everything needed to dial a remote host.
type ConnInfo struct {
	User       string `json:"user"`
	Addr       string `json:"addr"`
	Port       int    `json:"port"`
	AuthMode   string `json:"authMode"`
	Password   string `json:"password"`
	PrivateKey []byte `json:"privateKey"`
	PassPhrase []byte `json:"passPhrase"`

	UseProxy    bool          `json:"useProxy"`
	DialTimeOut time.Duration `json:"dialTimeOut"`
}

// SSHClient wraps gossh.Client with a SudoItem hint detected at dial time.
type SSHClient struct {
	Client   *gossh.Client `json:"client"`
	SudoItem string        `json:"sudoItem"`
}

// Logger is the minimal logging interface libpanel/ssh emits to. Optional.
type Logger interface {
	Errorf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

// ProxyConfig describes an outbound proxy to use for SSH dials. Returned
// by a ProxyResolver supplied by the host app. The Type field selects
// which dialer to use:
//   - "http" or "https" → HTTPProxyDialer (CONNECT method)
//   - "socks5"          → golang.org/x/net/proxy.SOCKS5
//   - "" (empty)        → no proxy, direct dial
type ProxyConfig struct {
	Type     string
	URL      string
	Port     string
	User     string
	Password string // plain text — caller is responsible for decryption
}

// ProxyResolver returns the currently configured outbound proxy. Called
// each time a dial happens (so config changes pick up without restart).
// Returning an empty Type or an error means no proxy will be used.
type ProxyResolver func() (*ProxyConfig, error)

var (
	log           Logger
	proxyResolver ProxyResolver
)

// SetLogger registers an optional logger. Without one, internal Debugf /
// Errorf calls are no-ops.
func SetLogger(l Logger) { log = l }

// SetProxyResolver registers an optional outbound-proxy resolver.
// Without one, dials are always direct (UseProxy is ignored).
func SetProxyResolver(r ProxyResolver) { proxyResolver = r }

func NewClient(c ConnInfo) (*SSHClient, error) {
	config := &gossh.ClientConfig{}
	config.SetDefaults()
	addr := net.JoinHostPort(c.Addr, fmt.Sprintf("%d", c.Port))
	config.User = c.User
	if c.AuthMode == "password" {
		config.Auth = []gossh.AuthMethod{gossh.Password(c.Password)}
	} else {
		signer, err := makePrivateKeySigner(c.PrivateKey, c.PassPhrase)
		if err != nil {
			return nil, err
		}
		config.Auth = []gossh.AuthMethod{gossh.PublicKeys(signer)}
	}
	if c.DialTimeOut == 0 {
		c.DialTimeOut = 5 * time.Second
	}
	config.Timeout = c.DialTimeOut

	config.HostKeyCallback = gossh.InsecureIgnoreHostKey()
	proto := "tcp"
	if strings.Contains(c.Addr, ":") {
		proto = "tcp6"
	}
	client, err := DialWithTimeout(proto, addr, c.UseProxy, config)
	if nil != err {
		return nil, err
	}
	sshClient := &SSHClient{Client: client}
	if c.User == "root" {
		return sshClient, nil
	}
	if _, err := sshClient.Run("sudo -n ls"); err == nil {
		sshClient.SudoItem = "sudo"
	}
	return sshClient, nil
}

func (c *SSHClient) Run(shell string) (string, error) {
	shell = c.SudoItem + " " + shell
	shell = strings.ReplaceAll(shell, " && ", fmt.Sprintf(" && %s ", c.SudoItem))
	session, err := c.Client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	buf, err := session.CombinedOutput(shell)

	return string(buf), err
}

func (c *SSHClient) CpFileWithCheck(src, dst string) error {
	localMd5, err := c.Runf("md5sum %s | awk '{print $1}'", src)
	if err != nil {
		if log != nil {
			log.Debugf("load md5sum with src for %s failed, std: %s, err: %v", path.Base(src), localMd5, err)
		}
		localMd5 = ""
	}
	for i := 0; i < 3; i++ {
		std, cpErr := c.Runf("cp %s %s", src, dst)
		if err != nil {
			err = fmt.Errorf("cp file %s failed, std: %s, err: %v", src, std, cpErr)
			continue
		}
		if len(strings.TrimSpace(localMd5)) == 0 {
			return nil
		}
		remoteMd5, errDst := c.Runf("md5sum %s | awk '{print $1}'", dst)
		if errDst != nil {
			if log != nil {
				log.Debugf("load md5sum with dst for %s failed, std: %s, err: %v", path.Base(src), remoteMd5, errDst)
			}
			return nil
		}
		if strings.TrimSpace(localMd5) == strings.TrimSpace(remoteMd5) {
			return nil
		}
		err = errors.New("cp file failed, file is not match!")
	}

	return err
}

func (c *SSHClient) SudoHandleCmd() string {
	if _, err := c.Run("sudo -n ls"); err == nil {
		return "sudo "
	}
	return ""
}

func (c *SSHClient) IsRoot(user string) bool {
	if user == "root" {
		return true
	}
	_, err := c.Run("sudo -n true")
	return err == nil
}

func (c *SSHClient) Runf(shell string, args ...interface{}) (string, error) {
	shell = c.SudoItem + " " + shell
	shell = strings.ReplaceAll(shell, " && ", fmt.Sprintf(" && %s ", c.SudoItem))
	session, err := c.Client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	buf, err := session.CombinedOutput(fmt.Sprintf(shell, args...))

	return string(buf), err
}

func (c *SSHClient) Close() {
	if c.Client != nil {
		_ = c.Client.Close()
	}
}

func makePrivateKeySigner(privateKey []byte, passPhrase []byte) (gossh.Signer, error) {
	if len(passPhrase) != 0 {
		return gossh.ParsePrivateKeyWithPassphrase(privateKey, passPhrase)
	}
	return gossh.ParsePrivateKey(privateKey)
}

func (c *SSHClient) RunWithStreamOutput(command string, outputCallback func(string)) error {
	session, err := c.Client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to set up stderr pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	stdoutCh := make(chan string, 100)
	stderrCh := make(chan string, 100)
	doneCh := make(chan struct{})

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := stdout.Read(buffer)
			if err != nil {
				close(stdoutCh)
				return
			}
			if n > 0 {
				stdoutCh <- string(buffer[:n])
			}
		}
	}()

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := stderr.Read(buffer)
			if err != nil {
				close(stderrCh)
				return
			}
			if n > 0 {
				stderrCh <- string(buffer[:n])
			}
		}
	}()

	go func() {
		for {
			select {
			case stdoutOutput, ok := <-stdoutCh:
				if !ok {
					stdoutCh = nil
					if stderrCh == nil {
						close(doneCh)
						return
					}
					continue
				}
				if outputCallback != nil {
					outputCallback(stdoutOutput)
				}

			case stderrOutput, ok := <-stderrCh:
				if !ok {
					stderrCh = nil
					if stdoutCh == nil {
						close(doneCh)
						return
					}
					continue
				}
				if outputCallback != nil {
					outputCallback(stderrOutput)
				}
			}
		}
	}()

	err = session.Wait()
	<-doneCh

	return err
}

func DialWithTimeout(network, addr string, useProxy bool, config *gossh.ClientConfig) (*gossh.Client, error) {
	var conn net.Conn
	var err error
	if useProxy {
		conn, err = loadSSHConnByProxy(network, addr, config.Timeout)
	} else {
		conn, err = net.DialTimeout(network, addr, config.Timeout)
	}
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(config.Timeout))
	c, chans, reqs, err := gossh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("clear deadline failed: %v", err)
	}
	return gossh.NewClient(c, chans, reqs), nil
}

func loadSSHConnByProxy(network, addr string, timeout time.Duration) (net.Conn, error) {
	if proxyResolver == nil {
		// No proxy resolver registered → fall back to direct dial.
		return net.DialTimeout(network, addr, timeout)
	}
	cfg, err := proxyResolver()
	if err != nil {
		return nil, fmt.Errorf("resolve proxy config failed: %v", err)
	}
	if cfg == nil || cfg.Type == "" {
		return net.DialTimeout(network, addr, timeout)
	}
	proxyItem := fmt.Sprintf("%s:%s", cfg.URL, cfg.Port)
	switch cfg.Type {
	case "http", "https":
		item := HTTPProxyDialer{
			Type:     cfg.Type,
			URL:      proxyItem,
			User:     cfg.User,
			Password: cfg.Password,
		}
		return HTTPDial(item, network, addr)
	case "socks5":
		var auth *proxy.Auth
		if len(cfg.User) != 0 {
			auth = &proxy.Auth{
				User:     cfg.User,
				Password: cfg.Password,
			}
		}
		dialer, err := proxy.SOCKS5("tcp", proxyItem, auth, &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("new socks5 proxy failed, err: %v", err)
		}
		return dialer.Dial(network, addr)
	default:
		return net.DialTimeout(network, addr, timeout)
	}
}
