package artifacts

import (
	"archive/tar"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ExtractLimits struct {
	MaxTotalBytes int64
	MaxFiles      int
	MaxFileBytes  int64
}

func (l ExtractLimits) withDefaults() ExtractLimits {
	// 0 means unlimited.
	if l.MaxTotalBytes < 0 {
		l.MaxTotalBytes = 0
	}
	if l.MaxFiles < 0 {
		l.MaxFiles = 0
	}
	if l.MaxFileBytes < 0 {
		l.MaxFileBytes = 0
	}
	return l
}

func ExtractTarToDir(r io.Reader, destDir string) error {
	return ExtractTarToDirWithLimits(r, destDir, ExtractLimits{})
}

func ExtractTarToDirWithLimits(r io.Reader, destDir string, limits ExtractLimits) error {
	limits = limits.withDefaults()

	tr := tar.NewReader(r)
	var extractedFiles int
	var extractedBytes int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		raw := strings.ReplaceAll(hdr.Name, "\\", "/")
		comps := strings.Split(raw, "/")
		skip := false
		for _, c := range comps {
			if c == ".." {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		name := filepath.Clean(hdr.Name)
		// Docker cp usually prefixes with the base directory; strip first path element.
		if name == "." {
			continue
		}
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) > 1 {
			name = filepath.Join(parts[1:]...)
		} else {
			name = parts[0]
		}
		name = filepath.Clean(name)
		if name == "." || name == "" {
			continue
		}
		if strings.HasPrefix(name, ".."+string(filepath.Separator)) || name == ".." {
			continue
		}

		outPath := filepath.Join(destDir, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			extractedFiles++
			if limits.MaxFiles > 0 && extractedFiles > limits.MaxFiles {
				return errors.New("artifact extraction: too many files")
			}

			if hdr.Size < 0 {
				return errors.New("artifact extraction: negative file size")
			}
			if limits.MaxFileBytes > 0 && hdr.Size > limits.MaxFileBytes {
				return errors.New("artifact extraction: file too large")
			}
			if limits.MaxTotalBytes > 0 && extractedBytes+hdr.Size > limits.MaxTotalBytes {
				return errors.New("artifact extraction: total size exceeded")
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}

			// Enforce per-file size while copying (hdr.Size can be incorrect).
			maxFileBytes := hdr.Size
			if limits.MaxFileBytes > 0 && maxFileBytes > limits.MaxFileBytes {
				maxFileBytes = limits.MaxFileBytes
			}
			written, copyErr := io.CopyN(f, tr, maxFileBytes)
			if copyErr == io.EOF {
				copyErr = io.ErrUnexpectedEOF
			}
			if copyErr != nil {
				_ = f.Close()
				return copyErr
			}
			if written != hdr.Size {
				_ = f.Close()
				if limits.MaxFileBytes > 0 && hdr.Size > limits.MaxFileBytes {
					return errors.New("artifact extraction: file too large")
				}
				return errors.New("artifact extraction: short write")
			}
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}

			extractedBytes += hdr.Size
		default:
			// ignore symlinks and other types
		}
	}
}

func ListFiles(root string) ([]struct {
	Path string
	Size int64
}, error) {
	var files []struct {
		Path string
		Size int64
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, struct {
			Path string
			Size int64
		}{Path: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
