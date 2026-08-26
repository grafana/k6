package xk6ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

// K6SSH is the main export of the k6 extension.
type K6SSH struct {
	Session *ssh.Session
	Client  *ssh.Client
	Config  *ssh.ClientConfig
	Out     *bytes.Buffer
	Stdin   io.WriteCloser
	fs      afero.Fs
}

// ConnectionOptions provides configuration for the SSH session.
type ConnectionOptions struct {
	RsaKey     string
	PrivateKey string //nolint:gosec // user-supplied private key contents option, not a hardcoded credential
	Passphrase string
	Host       string
	Port       int
	Username   string
	Password   string //nolint:gosec // user-supplied connection password option, not a hardcoded credential
}

// Connect starts and SSH session with the provided options.
func (k6ssh *K6SSH) Connect(options ConnectionOptions) error {
	var (
		authMethod ssh.AuthMethod
		err        error
	)

	if options.Password != "" {
		authMethod = ssh.Password(options.Password)
	} else {
		authMethod, err = k6ssh.rsaKeyAuthMethod(options)
		if err != nil {
			return err
		}
	}

	k6ssh.Config = &ssh.ClientConfig{
		Config: ssh.Config{},
		User:   options.Username,
		Auth:   []ssh.AuthMethod{authMethod},
		// #nosec G106
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		ClientVersion:   "",
		Timeout:         0,
	}

	addr := fmt.Sprintf("%s:%d", options.Host, options.Port)

	client, err := ssh.Dial("tcp", addr, k6ssh.Config)
	if err != nil {
		return err
	}

	k6ssh.Client = client

	return nil
}

// Run executes a remote command over SSH.
func (k6ssh *K6SSH) Run(command string) (string, error) {
	session, err := k6ssh.Client.NewSession()
	if err != nil {
		return "", err
	}

	defer func() {
		_ = session.Close()
	}()

	var stdoutBuf bytes.Buffer

	session.Stdout = &stdoutBuf
	err = session.Run(command)

	return stdoutBuf.String(), err
}

func (k6ssh *K6SSH) rsaKeyAuthMethod(options ConnectionOptions) (ssh.AuthMethod, error) {
	var key []byte
	var err error

	if options.PrivateKey != "" {
		key = []byte(options.PrivateKey)
	} else {
		pk := options.RsaKey
		if pk == "" {
			pk = k6ssh.defaultKeyPath()
		}

		key, err = afero.ReadFile(k6ssh.fs, pk)
		if err != nil {
			return nil, err
		}
	}

	var signer ssh.Signer
	if options.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(options.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(key)
	}

	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}

func (k6ssh *K6SSH) defaultKeyPath() string {
	home := os.Getenv("HOME") //nolint:forbidigo // locating the user's default SSH key requires the home dir
	if len(home) > 0 {
		return path.Join(home, ".ssh/id_rsa")
	}

	return ""
}
