package store

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
)

type BadgerStore struct {
	db *badger.DB
}

func OpenBadger(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)

	opts.SyncWrites = false // I disable sync writes for maximum throughput.
	opts.NumMemtables = 5
	opts.NumLevelZeroTables = 5
	opts.NumLevelZeroTablesStall = 10
	opts.ValueLogFileSize = 1 << 30 // I set the value log file size to 1GB.
	opts.ValueLogMaxEntries = 1000000
	opts.NumCompactors = 4            // I tune this for available CPU.
	opts.Compression = options.Snappy // I prefer Snappy; ZSTD costs more CPU.
	opts.BlockCacheSize = 512 << 20   // I set the block cache to 512MB.
	opts.IndexCacheSize = 256 << 20

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerStore{db: db}, nil
}

func (s *BadgerStore) Close() error {
	return s.db.Close()
}

func (s *BadgerStore) ListKeysPage(startAfter string, limit int) ([]string, string, bool, error) {
	if limit <= 0 {
		return nil, "", false, nil
	}
	var keys []string
	var lastKey string
	var hasMore bool
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		if startAfter == "" {
			it.Rewind()
		} else {
			after := []byte(startAfter)
			it.Seek(after)
			if it.Valid() && bytes.Equal(it.Item().Key(), after) {
				it.Next()
			}
		}

		for it.Valid() {
			item := it.Item()
			key := string(item.KeyCopy(nil))
			keys = append(keys, key)
			lastKey = key
			if len(keys) >= limit {
				it.Next()
				if it.Valid() {
					hasMore = true
				}
				break
			}
			it.Next()
		}
		return nil
	})
	return keys, lastKey, hasMore, err
}

func (s *BadgerStore) CountKeysMatching(term string) (int, error) {
	pattern := strings.TrimSpace(term)
	if pattern == "" {
		return 0, nil
	}
	pr := []rune(pattern)
	count := 0
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if fuzzyMatch(pr, key) {
				count++
			}
		}
		return nil
	})
	return count, err
}

func (s *BadgerStore) GroupKeyCounts() (map[string]int, error) {
	counts := make(map[string]int)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			group := keyGroup(key)
			counts[group]++
		}
		return nil
	})
	return counts, err
}

func keyGroup(key string) string {
	if key == "" {
		return "(empty)"
	}
	if idx := strings.IndexByte(key, ':'); idx > 0 {
		return key[:idx]
	}
	return "(no prefix)"
}

func fuzzyMatch(pattern []rune, target string) bool {
	if len(pattern) == 0 {
		return true
	}
	pi := 0
	for _, r := range target {
		if equalFold(r, pattern[pi]) {
			pi++
			if pi == len(pattern) {
				return true
			}
		}
	}
	return false
}

// I lifted this from strings.EqualFold.
func equalFold(tr, sr rune) bool {
	if tr == sr {
		return true
	}
	if tr < sr {
		tr, sr = sr, tr
	}
	// I fast-path ASCII.
	if tr < utf8.RuneSelf {
		// I normalize ASCII case by comparing lower/upper pairs.
		if 'A' <= sr && sr <= 'Z' && tr == sr+'a'-'A' {
			return true
		}
		return false
	}

	// I fall back to SimpleFold for the general case, which cycles equivalents.
	r := unicode.SimpleFold(sr)
	for r != sr && r < tr {
		r = unicode.SimpleFold(r)
	}
	return r == tr
}

func (s *BadgerStore) Get(key string) ([]byte, error) {
	var out []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			out = append(out, v...)
			return nil
		})
	})
	return out, err
}

func (s *BadgerStore) Set(key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

func (s *BadgerStore) Delete(key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}
