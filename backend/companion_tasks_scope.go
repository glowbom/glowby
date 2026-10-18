package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func companionTasksScope(salt []byte, installationID, uid string) string {
	encoded, _ := json.Marshal([]string{hex.EncodeToString(salt), installationID, uid})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (s *companionSession) tasksScope(installationID, uid string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tasksScopeSalt) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		salt, err := readCompanionTasksScopeSalt(filepath.Join(home, ".glowbom", "desktop", "companion-tasks", "scope"))
		if err != nil {
			return "", err
		}
		s.tasksScopeSalt = salt
	}
	return companionTasksScope(s.tasksScopeSalt, installationID, uid), nil
}

// Publish a complete private file atomically. Competing backend launches read
// the winner's salt, so they cannot give the same inbox different identities.
func readCompanionTasksScopeSalt(path string) ([]byte, error) {
	invalid := errors.New("private Tasks scope is unavailable")
	directory := filepath.Dir(path)
	if !filepath.IsAbs(path) || os.MkdirAll(directory, 0o700) != nil {
		return nil, invalid
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, invalid
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, invalid
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0o077 != 0 {
		return nil, invalid
	}
	name := filepath.Base(path)
	if _, err := root.Lstat(name); errors.Is(err, os.ErrNotExist) {
		salt := make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, invalid
		}
		temporary := ".companion-tasks-scope-" + hex.EncodeToString(salt[:16])
		file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, invalid
		}
		defer root.Remove(temporary)
		_, writeError := file.Write(salt)
		syncError := file.Sync()
		closeError := file.Close()
		if writeError != nil || syncError != nil || closeError != nil {
			return nil, invalid
		}
		if err := root.Link(temporary, name); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, invalid
		}
	}
	info, err = root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() != 32 {
		return nil, invalid
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, invalid
	}
	defer file.Close()
	opened, err = file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0o077 != 0 {
		return nil, invalid
	}
	salt, err := io.ReadAll(io.LimitReader(file, 33))
	if err != nil || len(salt) != 32 {
		return nil, invalid
	}
	return salt, nil
}
