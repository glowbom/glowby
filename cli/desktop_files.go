package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const desktopAttachmentLimit = 8 << 20
const desktopAttachmentsLimit = 16 << 20
const desktopTextLimit = 128 << 10

type desktopAttachment struct {
	Name       string `json:"name"`
	MimeType   string `json:"mimeType"`
	DataBase64 string `json:"dataBase64"`
	SHA256     string `json:"sha256"`
}

type desktopSavedAttachment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

func saveDesktopAttachment(ctx context.Context, result json.RawMessage, expectedID, output string) (json.RawMessage, error) {
	var downloaded struct {
		Attachment desktopSavedAttachment `json:"attachment"`
		DataBase64 string                 `json:"dataBase64"`
	}
	if json.Unmarshal(result, &downloaded) != nil || downloaded.Attachment.ID != expectedID ||
		downloaded.Attachment.SizeBytes < 0 || downloaded.Attachment.SizeBytes > desktopAttachmentLimit ||
		len(downloaded.DataBase64) > base64.StdEncoding.EncodedLen(desktopAttachmentLimit) {
		return nil, errors.New("Desktop returned an invalid attachment")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(downloaded.DataBase64)
	if err != nil || int64(len(data)) != downloaded.Attachment.SizeBytes || len(data) > desktopAttachmentLimit {
		return nil, errors.New("the attachment download is incomplete; try again")
	}
	digest := sha256.Sum256(data)
	if downloaded.Attachment.SHA256 != hex.EncodeToString(digest[:]) {
		return nil, errors.New("the attachment checksum did not match; no output was saved")
	}
	absolute, err := filepath.Abs(output)
	if err != nil {
		return nil, errors.New("could not resolve the attachment output path")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return nil, errors.New("the attachment output parent must be an existing directory")
	}
	destination := filepath.Join(parent, filepath.Base(absolute))
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("the attachment output already exists or cannot be inspected; choose a new path")
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.New("attachment download cancelled; no output was saved")
	}
	file, err := os.CreateTemp(parent, ".glowbom-task-file-*")
	if err != nil {
		return nil, errors.New("could not prepare the attachment output; choose a writable directory")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return nil, errors.New("could not make the attachment output private")
	}
	if _, err := file.Write(data); err != nil {
		return nil, errors.New("could not write the complete attachment")
	}
	if err := file.Sync(); err != nil {
		return nil, errors.New("could not finish writing the attachment")
	}
	if err := file.Close(); err != nil {
		return nil, errors.New("could not close the attachment output")
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.New("attachment download cancelled; no output was saved")
	}
	// A hard link publishes the complete file and cannot replace an existing path.
	if err := os.Link(file.Name(), destination); err != nil {
		return nil, errors.New("could not save the attachment without replacing another file; choose a new path")
	}
	return json.Marshal(struct {
		Attachment desktopSavedAttachment `json:"attachment"`
		Output     string                 `json:"output"`
	}{Attachment: downloaded.Attachment, Output: destination})
}

func readDesktopFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("choose a readable regular file, not a folder or symbolic link")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("the file exceeds the %d byte limit", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("could not open the local file")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("the file changed while opening; try again")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("could not read the complete file within its size limit")
	}
	finished, err := file.Stat()
	if err != nil || finished.Size() != opened.Size() || !finished.ModTime().Equal(opened.ModTime()) || int64(len(data)) != opened.Size() {
		return nil, errors.New("the file changed while reading; save it and try again")
	}
	return data, nil
}

func readDesktopText(path string) (string, error) {
	data, err := readDesktopFile(path, desktopTextLimit)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		return "", errors.New("choose a UTF-8 text file")
	}
	return string(data), nil
}

func readDesktopAttachments(paths []string) ([]desktopAttachment, error) {
	if len(paths) > 8 {
		return nil, errors.New("attach at most 8 files per request")
	}
	attachments := make([]desktopAttachment, 0, len(paths))
	total := 0
	for index, path := range paths {
		data, err := readDesktopFile(path, desktopAttachmentLimit)
		if err != nil {
			return nil, fmt.Errorf("attachment %d: %w", index+1, err)
		}
		total += len(data)
		if total > desktopAttachmentsLimit {
			return nil, errors.New("attachments together exceed 16 MiB")
		}
		name := filepath.Base(path)
		if !utf8.ValidString(name) || len(name) > 240 || strings.ContainsAny(name, "\\\x00\r\n") {
			return nil, fmt.Errorf("attachment %d needs a shorter plain filename", index+1)
		}
		mediaType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		if mediaType == "" {
			mediaType = http.DetectContentType(data)
		}
		mediaType, _, _ = mime.ParseMediaType(mediaType)
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		digest := sha256.Sum256(data)
		attachments = append(attachments, desktopAttachment{
			Name: name, MimeType: mediaType, DataBase64: base64.StdEncoding.EncodeToString(data), SHA256: hex.EncodeToString(digest[:]),
		})
	}
	return attachments, nil
}
