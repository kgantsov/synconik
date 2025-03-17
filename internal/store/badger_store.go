package store

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/kgantsov/synconic/internal/entity"
	"github.com/rs/zerolog/log"
)

const (
	FILES_BUCKET = "files"
)

type BadgerStore struct {
	db *badger.DB
}

// not found error
var ErrFileNotFound = fmt.Errorf("file not found")

func NewBadgerStore(dir string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dir)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerStore{db: db}, nil
}

// Close closes the BadgerStore instance
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// Set sets a key-value pair in the specified bucket
func (s *BadgerStore) Set(bucket, key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(getKey(bucket, key), value)
	})
}

// Get retrieves a value for a given key in the specified bucket
func (s *BadgerStore) Get(bucket, key string) ([]byte, error) {
	var valCopy []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(getKey(bucket, key))
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		return err
	})
	return valCopy, err
}

// Delete removes a key-value pair from the specified bucket
func (s *BadgerStore) Delete(bucket, key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(getKey(bucket, key))
	})
}

func (s *BadgerStore) ExistsFile(path string) (bool, error) {
	_, err := s.Get(FILES_BUCKET, path)
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) GetFile(path string) (*entity.File, error) {
	data, err := s.Get(FILES_BUCKET, path)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrFileNotFound
		}
		return nil, err
	}

	file := &entity.File{}
	if err := file.Unmarshal(data); err != nil {
		return nil, err
	}

	return file, nil
}

func (s *BadgerStore) SaveFile(path string, file *entity.File) error {
	log.Debug().Str("service", "store").Msgf("Saving file %s", path)

	data, err := file.Marshal()
	if err != nil {
		return err
	}

	return s.Set(FILES_BUCKET, path, data)
}

func (s *BadgerStore) DeleteFile(path string) error {
	return s.Delete(FILES_BUCKET, path)
}

// getKey generates the key with the bucket prefix
func getKey(bucket, key string) []byte {
	return []byte(fmt.Sprintf("%s:%s", bucket, key))
}
