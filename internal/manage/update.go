package manage

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Update replaces the installed binary from the latest release and rolls back
// if restart or health-check fails.
func Update(opts Options) error {
	opts = opts.withDefaults()
	if err := requireLinuxRoot(opts.Root); err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return errors.New("update is only supported on Linux")
	}
	target, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return err
	}
	latest, err := fetchLatest(opts)
	if err != nil {
		return err
	}
	if sameReleaseVersion(opts.Version, latest.TagName) {
		fmt.Fprintf(opts.Stdout, "already up to date: %s\n", latest.TagName)
		return nil
	}
	archiveName, err := releaseArchiveName(latest.TagName)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "sb-fox-update-*")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(tmp)

	fmt.Fprintf(opts.Stdout, "latest version: %s\n", latest.TagName)
	archivePath := filepath.Join(tmp, archiveName)
	sumPath := filepath.Join(tmp, "SHA256SUMS")
	archiveAsset, err := releaseAssetByName(latest.Assets, archiveName)
	if err != nil {
		return err
	}
	sumAsset, err := releaseAssetByName(latest.Assets, "SHA256SUMS")
	if err != nil {
		return err
	}
	if err := downloadAsset(opts, archiveAsset.ID, archivePath, "download archive"); err != nil {
		return err
	}
	if err := downloadAsset(opts, sumAsset.ID, sumPath, "download checksum"); err != nil {
		return err
	}
	if err := verifySHA256(archivePath, sumPath, archiveName); err != nil {
		return err
	}
	fmt.Fprintln(opts.Stdout, "checksum verified")
	if err := extractTarGz(archivePath, tmp); err != nil {
		return err
	}
	newBinary, err := findBinary(tmp)
	if err != nil {
		return err
	}
	backup := target + ".bak-" + time.Now().UTC().Format("20060102-150405")
	if err := copyFile(target, backup, 0o755); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "backup created: %s\n", filepath.Base(backup))
	if err := replaceBinary(target, newBinary); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "binary replaced")

	if err := restartAndCheck(opts); err != nil {
		fmt.Fprintln(opts.Stdout, "health-check failed, rolling back")
		if rbErr := replaceBinary(target, backup); rbErr != nil {
			return fmt.Errorf("rollback failed after update error: %v; rollback: %w", err, rbErr)
		}
		if checkErr := restartAndCheck(opts); checkErr != nil {
			return fmt.Errorf("rollback health-check failed after update error: %v; rollback check: %w", err, checkErr)
		}
		return fmt.Errorf("update rolled back: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return fmt.Errorf("remove update backup: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "update completed")
	return nil
}

func verifySHA256(archivePath, sumPath, archiveName string) error {
	want, err := checksumFor(sumPath, archiveName)
	if err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("read archive for checksum: %w", err)
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return errors.New("checksum verification failed")
	}
	return nil
}

func checksumFor(path, archiveName string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open checksum file: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == archiveName {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksum file: %w", err)
	}
	return "", errors.New("checksum entry not found")
}

func extractTarGz(path, dir string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("extract archive: %w", err)
		}
		clean := filepath.Clean(header.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return errors.New("archive contains unsafe path")
		}
		target := filepath.Join(dir, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create archive directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create archive parent: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create archive file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return fmt.Errorf("write archive file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close archive file: %w", err)
			}
		}
	}
}

func findBinary(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "sb-fox" {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", fmt.Errorf("find release binary: %w", err)
	}
	if found == "" {
		return "", errors.New("release binary not found")
	}
	return found, nil
}

func replaceBinary(target, source string) error {
	tmp := target + ".new"
	if err := copyFile(source, tmp, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}
