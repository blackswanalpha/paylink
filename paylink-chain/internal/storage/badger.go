package storage

import (
	"github.com/dgraph-io/badger/v4"
)

// BadgerStore implements KVStore using BadgerDB.
type BadgerStore struct {
	db *badger.DB
}

// NewBadgerStore opens or creates a BadgerDB at the given path.
func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Suppress BadgerDB's default logging

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) Get(key []byte) ([]byte, error) {
	var val []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return val, err
}

func (s *BadgerStore) Set(key, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (s *BadgerStore) Delete(key []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (s *BadgerStore) Has(key []byte) (bool, error) {
	var exists bool
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

func (s *BadgerStore) NewBatch() Batch {
	return &badgerBatch{db: s.db, wb: s.db.NewWriteBatch()}
}

func (s *BadgerStore) Iterate(prefix []byte, fn func(key, value []byte) bool) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if !fn(item.Key(), val) {
				break
			}
		}
		return nil
	})
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// badgerBatch implements the Batch interface.
type badgerBatch struct {
	db *badger.DB
	wb *badger.WriteBatch
}

func (b *badgerBatch) Set(key, value []byte) {
	_ = b.wb.Set(key, value)
}

func (b *badgerBatch) Delete(key []byte) {
	_ = b.wb.Delete(key)
}

func (b *badgerBatch) Flush() error {
	return b.wb.Flush()
}
