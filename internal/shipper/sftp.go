package shipper

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var _ remoteFS = (*SFTP)(nil)

// SFTP implements remoteFS on top of an SFTP connection
type SFTP struct {
	client    *sftp.Client
	sshClient *ssh.Client
}

func (s *SFTP) MkdirAll(p string) error {
	return s.client.MkdirAll(p)
}

func (s *SFTP) Create(p string) (io.WriteCloser, error) {
	f, err := s.client.Create(p)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *SFTP) Open(p string) (io.ReadCloser, error) {
	f, err := s.client.Open(p)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *SFTP) Rename(oldPath, newPath string) error {
	// some servers (e.g. Hetzner Storage Boxes) overwrite existing targets on rename, others refuse it. check first
	// to make sure the behavior is the same everywhere.
	if _, err := s.client.Stat(newPath); err == nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: fs.ErrExist}
	}

	err := s.client.Rename(oldPath, newPath)
	if err == nil {
		return nil
	}
	// sftp reports an existing target only as a generic SSH_FX_FAILURE, so we check the target explicitly
	if _, statErr := s.client.Stat(newPath); statErr == nil {
		return &os.LinkError{Op: "rename", Old: oldPath, New: newPath, Err: fs.ErrExist}
	}
	return err
}

func (s *SFTP) Remove(p string) error {
	return s.client.Remove(p)
}

// DialSFTP establishes a new sftp connection to a remote server.
func DialSFTP(addr, user, keyPath, knownHostsPath string) (*SFTP, error) {
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read ssh key %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s: %w", keyPath, err)
	}

	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known hosts %s: %w", knownHostsPath, err)
	}

	config := &ssh.ClientConfig{
		// timeout is only applicable for the connection and initial handshake
		// the timeout during the upload must be handled via systemd
		Timeout:         15 * time.Second,
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
	}

	sshClient, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("start sftp session: %w", err)
	}

	return &SFTP{
		client:    sftpClient,
		sshClient: sshClient,
	}, nil
}

// Close ends the connection to the sftp server.
func (s *SFTP) Close() error {
	var err error
	if s.client != nil {
		err = s.client.Close()
	}
	if s.sshClient != nil {
		err = errors.Join(err, s.sshClient.Close())
	}
	return err
}
