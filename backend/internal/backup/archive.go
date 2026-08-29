package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	maximumCompressedBytes = 2 << 30
	maximumExpandedBytes   = 8 << 30
	maximumArchiveEntries  = 100_000
)

func writeArchive(ctx context.Context, output io.Writer, homePath string, manifest Manifest, createdAt time.Time) error {
	gzipWriter, err := gzip.NewWriterLevel(output, gzip.DefaultCompression)
	if err != nil {
		return err
	}
	tarWriter := tar.NewWriter(gzipWriter)
	closeWriters := func() error { return errors.Join(tarWriter.Close(), gzipWriter.Close()) }
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = closeWriters()
		return err
	}
	if err := writeTarBytes(tarWriter, ManifestPath, append(manifestBytes, '\n'), 0o600, createdAt); err != nil {
		_ = closeWriters()
		return err
	}
	help := "Stop NovelReader before manual restore. Copy the contents of reader-home/ into the target reader directory.\nThe target account credentials and API tokens remain outside this payload.\n"
	if err := writeTarBytes(tarWriter, RestoreHelp, []byte(help), 0o600, createdAt); err != nil {
		_ = closeWriters()
		return err
	}
	if err := filepath.WalkDir(homePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(homePath, path)
		if err != nil {
			return err
		}
		name := PayloadRoot
		if relative != "." {
			name += "/" + filepath.ToSlash(relative)
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return readerstore.ErrInvalidFilePath
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
		header.ModTime, header.AccessTime, header.ChangeTime = createdAt, time.Time{}, time.Time{}
		if entry.IsDir() {
			header.Mode = 0o700
			return tarWriter.WriteHeader(header)
		}
		if !info.Mode().IsRegular() {
			return readerstore.ErrInvalidFilePath
		}
		header.Mode = 0o600
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, file)
		return errors.Join(copyErr, file.Close())
	}); err != nil {
		_ = closeWriters()
		return err
	}
	return closeWriters()
}

func extractArchive(ctx context.Context, input io.Reader, destination string) (Manifest, string, error) {
	limited := &io.LimitedReader{R: input, N: maximumCompressedBytes + 1}
	gzipReader, err := gzip.NewReader(limited)
	if err != nil {
		return Manifest{}, "", fmt.Errorf("backup: open gzip: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var manifest Manifest
	manifestSeen := false
	expanded := int64(0)
	entries := 0
	seen := make(map[string]bool)
	for {
		if err := ctx.Err(); err != nil {
			return Manifest{}, "", err
		}
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, "", fmt.Errorf("backup: read tar: %w", err)
		}
		entries++
		if entries > maximumArchiveEntries || header.Size < 0 || expanded+header.Size > maximumExpandedBytes {
			return Manifest{}, "", fmt.Errorf("backup: archive limits exceeded")
		}
		expanded += header.Size
		name, err := safeArchivePath(header.Name)
		if err != nil || seen[name] {
			return Manifest{}, "", fmt.Errorf("backup: unsafe or duplicate archive entry")
		}
		seen[name] = true
		switch {
		case name == ManifestPath && header.Typeflag == tar.TypeReg:
			if header.Size > 64<<10 {
				return Manifest{}, "", fmt.Errorf("backup: manifest is too large")
			}
			decoder := json.NewDecoder(io.LimitReader(tarReader, header.Size))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&manifest); err != nil {
				return Manifest{}, "", fmt.Errorf("backup: invalid manifest: %w", err)
			}
			manifestSeen = true
		case name == RestoreHelp && header.Typeflag == tar.TypeReg:
			if _, err := io.Copy(io.Discard, io.LimitReader(tarReader, header.Size)); err != nil {
				return Manifest{}, "", err
			}
		case name == PayloadRoot && header.Typeflag == tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(destination, PayloadRoot), 0o700); err != nil {
				return Manifest{}, "", err
			}
		case strings.HasPrefix(name, PayloadRoot+"/"):
			target := filepath.Join(destination, filepath.FromSlash(name))
			if header.Typeflag == tar.TypeDir {
				if err := os.MkdirAll(target, 0o700); err != nil {
					return Manifest{}, "", err
				}
				continue
			}
			if header.Typeflag != tar.TypeReg {
				return Manifest{}, "", fmt.Errorf("backup: unsupported archive entry type")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return Manifest{}, "", err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return Manifest{}, "", err
			}
			_, copyErr := io.CopyN(file, tarReader, header.Size)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return Manifest{}, "", err
			}
		default:
			return Manifest{}, "", fmt.Errorf("backup: unexpected archive entry %q", name)
		}
	}
	if limited.N <= 0 || !manifestSeen {
		return Manifest{}, "", fmt.Errorf("backup: incomplete or oversized archive")
	}
	if manifest.Format != Format || manifest.FormatVersion != CurrentVersion {
		return Manifest{}, "", fmt.Errorf("backup: unsupported archive format")
	}
	return manifest, filepath.Join(destination, PayloadRoot), nil
}

func writeTarBytes(writer *tar.Writer, name string, data []byte, mode int64, modified time.Time) error {
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: modified, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func safeArchivePath(name string) (string, error) {
	name = strings.TrimSuffix(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", readerstore.ErrInvalidFilePath
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != name {
		return "", readerstore.ErrInvalidFilePath
	}
	return cleaned, nil
}
