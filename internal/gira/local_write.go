package gira

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type localWriteScope struct {
	root string
}

func newLocalWriteScope(root string) (localWriteScope, error) {
	if strings.TrimSpace(root) == "" {
		return localWriteScope{}, fmt.Errorf("local write root is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return localWriteScope{}, err
	}
	return localWriteScope{root: filepath.Clean(absRoot)}, nil
}

func writeSafeLocalFile(path string, data []byte, perm os.FileMode) error {
	scope, rel, err := localWriteScopeForPath(path)
	if err != nil {
		return err
	}
	return scope.WriteFile(rel, data, perm)
}

func appendSafeLocalFile(path string, data []byte, perm os.FileMode) error {
	scope, rel, err := localWriteScopeForPath(path)
	if err != nil {
		return err
	}
	return scope.AppendFile(rel, data, perm)
}

func prepareSafeLocalFile(path string) error {
	scope, rel, err := localWriteScopeForPath(path)
	if err != nil {
		return err
	}
	target, err := scope.resolve(rel)
	if err != nil {
		return err
	}
	return prepareSafeLocalFileTarget(scope.root, target)
}

func localWriteScopeForPath(path string) (localWriteScope, string, error) {
	if strings.TrimSpace(path) == "" {
		return localWriteScope{}, "", fmt.Errorf("local write path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return localWriteScope{}, "", err
	}
	scope, err := newLocalWriteScope(filepath.Dir(absPath))
	if err != nil {
		return localWriteScope{}, "", err
	}
	return scope, filepath.Base(absPath), nil
}

func (s localWriteScope) WriteFile(relPath string, data []byte, perm os.FileMode) error {
	target, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	if err := prepareSafeLocalFileTarget(s.root, target); err != nil {
		return err
	}
	return os.WriteFile(target, data, perm)
}

func (s localWriteScope) AppendFile(relPath string, data []byte, perm os.FileMode) error {
	target, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	if err := prepareSafeLocalFileTarget(s.root, target); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s localWriteScope) resolve(relPath string) (string, error) {
	if strings.TrimSpace(relPath) == "" {
		return "", fmt.Errorf("local write path is required")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("unsafe local write path %q: absolute paths are not allowed inside scoped writes", relPath)
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe local write path %q: path escapes write root", relPath)
	}
	target := filepath.Clean(filepath.Join(s.root, clean))
	if !pathWithinRoot(s.root, target) {
		return "", fmt.Errorf("unsafe local write path %q: path escapes write root", relPath)
	}
	return target, nil
}

func prepareSafeLocalFileTarget(root string, target string) error {
	if !pathWithinRoot(root, target) {
		return fmt.Errorf("unsafe local write path %q: path escapes write root", target)
	}
	if err := ensureSafeLocalDir(root); err != nil {
		return err
	}
	if err := ensureSafeLocalDir(filepath.Dir(target)); err != nil {
		return err
	}
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe local write path %q: symlink targets are not allowed", target)
	}
	if info.IsDir() {
		return fmt.Errorf("unsafe local write path %q: target is a directory", target)
	}
	return nil
}

func ensureSafeLocalDir(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absPath = filepath.Clean(absPath)
	root := filepath.VolumeName(absPath) + string(os.PathSeparator)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(current, 0o755); err != nil && !os.IsExist(err) {
					return err
				}
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe local write path %q: symlink directories are not allowed", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("unsafe local write path %q: parent is not a directory", current)
		}
	}
	return nil
}

func pathWithinRoot(root string, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
