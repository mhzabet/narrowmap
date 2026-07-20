package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"narrowmap/internal/model"
)

func loadSingleFile(path string, maxBody int64, stdin io.Reader) (model.Document, bool, error) {
	name := path
	var (
		body      []byte
		truncated bool
		err       error
	)
	if path == "-" {
		name = "stdin"
		body, truncated, err = readLimited(stdin, maxBody)
	} else {
		body, truncated, err = readLimitedFile(path, maxBody)
	}
	if err != nil {
		return model.Document{}, false, err
	}
	kind, err := detectKind(name, "", body)
	if err != nil {
		return model.Document{}, truncated, err
	}
	return model.Document{
		Name: name,
		Kind: kind,
		Body: body,
	}, truncated, nil
}

func listFolderFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && supportedFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func readLimitedFile(path string, limit int64) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	return readLimited(file, limit)
}

func readLimited(reader io.Reader, limit int64) ([]byte, bool, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > limit {
		return body[:limit], true, nil
	}
	return body, false, nil
}
