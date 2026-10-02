package shipper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"
)

var errRemoteMismatch = errors.New("remote file differs from local file")
var errUploadCorrupted = errors.New("uploaded file differs from local file")

type Shipper struct {
	localDir   string
	remote     remoteFS
	remoteRoot string
	now        func() time.Time
	log        *slog.Logger
}

type Report struct {
	Shipped    []string
	Failed     []string
	Ignored    []string
	Mismatches []string
}

type remoteFS interface {
	MkdirAll(path string) error
	Create(path string) (io.WriteCloser, error)
	Open(path string) (io.ReadCloser, error)
	Rename(oldPath string, newPath string) error
	Remove(path string) error
}

func New(localDir string, remote remoteFS, remoteRoot string, now func() time.Time, log *slog.Logger) *Shipper {
	return &Shipper{
		localDir:   localDir,
		remote:     remote,
		remoteRoot: remoteRoot,
		now:        now,
		log:        log,
	}
}

func (s *Shipper) Run(ctx context.Context) (Report, error) {
	names, err := s.localArchives()
	if err != nil {
		return Report{}, err
	}

	uploads, ignored := plan(names, s.now(), s.remoteRoot)
	if len(ignored) > 0 {
		s.log.Warn("ignoring unknown files", "files", ignored)
	}
	s.log.Info("starting run", "uploads", len(uploads))

	report := Report{Ignored: ignored}
	for _, u := range uploads {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := s.shipFile(u); err != nil {
			if errors.Is(err, errRemoteMismatch) {
				s.log.Error("remote archive differs from local archive", "file", u.LocalName, "err", err)
				report.Mismatches = append(report.Mismatches, u.LocalName)
				continue
			}
			s.log.Error("failed to ship archive", "file", u.LocalName, "err", err)
			report.Failed = append(report.Failed, u.LocalName)
			continue
		}
		s.log.Info("shipped archive", "file", u.LocalName, "remote", u.RemotePath)
		report.Shipped = append(report.Shipped, u.LocalName)
	}

	return report, nil
}

func (s *Shipper) localArchives() ([]string, error) {
	entries, err := os.ReadDir(s.localDir)
	if err != nil {
		return nil, fmt.Errorf("read local directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}

	return names, nil
}

func (s *Shipper) shipFile(u upload) error {
	localPath := filepath.Join(s.localDir, u.LocalName)

	existing, err := s.remote.Open(u.RemotePath)
	switch {
	// file dose not exists on remote
	case errors.Is(err, fs.ErrNotExist):
		if err := s.publish(localPath, u.RemotePath); err != nil {
			return err
		}
	case err != nil:
		return fmt.Errorf("open remote file %s: %w", u.RemotePath, err)
	// file has already been copied to remote but not deleted
	default:
		same, err := matchRemote(localPath, existing)
		if err != nil {
			return fmt.Errorf("compare with remote %s: %w", u.RemotePath, err)
		}
		if !same {
			return fmt.Errorf("%s: %w", u.RemotePath, errRemoteMismatch)
		}
	}

	if err := os.Remove(localPath); err != nil {
		return fmt.Errorf("remove local %s: %w", localPath, err)
	}
	return nil
}

func (s *Shipper) publish(localPath string, remotePath string) error {
	tmpPath := remotePath + ".tmp"

	if err := s.upload(localPath, tmpPath); err != nil {
		return err
	}
	if err := s.verify(localPath, tmpPath); err != nil {
		s.removeRemote(tmpPath)
		return err
	}
	if err := s.remote.Rename(tmpPath, remotePath); err != nil {
		s.removeRemote(tmpPath)
		return fmt.Errorf("publish %s: %w", remotePath, err)
	}
	return nil
}

func (s *Shipper) verify(localPath, remotePath string) error {
	uploaded, err := s.remote.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file %s: %w", remotePath, err)
	}
	same, err := matchRemote(localPath, uploaded)
	if err != nil {
		return fmt.Errorf("verify %s: %w", remotePath, err)
	}
	if !same {
		return fmt.Errorf("verify %s: %w", remotePath, errUploadCorrupted)
	}
	return nil
}

func matchRemote(localPath string, remote io.ReadCloser) (bool, error) {
	defer remote.Close()

	local, err := os.Open(localPath)
	if err != nil {
		return false, fmt.Errorf("open local file: %w", err)
	}
	defer local.Close()

	return sameContent(local, remote)
}

func sameContent(local io.Reader, remote io.Reader) (bool, error) {
	hl := sha256.New()
	if _, err := io.Copy(hl, local); err != nil {
		return false, fmt.Errorf("compare files: %w", err)
	}

	hr := sha256.New()
	if _, err := io.Copy(hr, remote); err != nil {
		return false, fmt.Errorf("compare files: %w", err)
	}

	same := bytes.Equal(hr.Sum(nil), hl.Sum(nil))

	return same, nil
}

func (s *Shipper) upload(localPath string, remotePath string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer src.Close()

	if err := s.remote.MkdirAll(path.Dir(remotePath)); err != nil {
		return fmt.Errorf("create remote directory: %w", err)
	}

	dst, err := s.remote.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote file %s: %w", remotePath, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		s.removeRemote(remotePath)
		return fmt.Errorf("copy remote file %s: %w", remotePath, err)
	}

	if err := dst.Close(); err != nil {
		s.removeRemote(remotePath)
		return fmt.Errorf("close remote file %s: %w", remotePath, err)
	}
	return nil
}

func (s *Shipper) removeRemote(p string) {
	if err := s.remote.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Warn("failed to remove remote file", "path", p, "err", err)
	}
}
