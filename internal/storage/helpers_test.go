package storage_test

import (
	"bytes"
	"io"

	"simplefs/internal/storage"
)

func writeTestFile(store *storage.FileStore, directory, name string, content []byte) error {
	return writeTestStream(store, directory, name, bytes.NewReader(content))
}

func writeTestStream(store *storage.FileStore, directory, name string, reader io.Reader) error {
	_, err := store.WriteFile(directory, name, reader, storage.WriteOptions{Conflict: storage.ConflictRename})
	return err
}
