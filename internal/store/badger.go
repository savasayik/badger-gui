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

	opts.SyncWrites = false // Disable sync writes for maximum throughput.
	opts.NumMemtables = 5
	opts.NumLevelZeroTables = 5
	opts.NumLevelZeroTablesStall = 10
	opts.ValueLogFileSize = 1 << 30 // Value log file size: 1GB.
	opts.ValueLogMaxEntries = 1000000
	opts.NumCompactors = 4            // Tuned for available CPU.
	opts.Compression = options.Snappy // Snappy over ZSTD; lower CPU cost.
	opts.BlockCacheSize = 512 << 20   // Block cache: 512MB.
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

// TreeGroupCounts iterates all keys and counts every `:` separated prefix up to maxDepth.
// The last segment is never counted as a group — it is the leaf value, not a prefix.
// For example, "log:error:auth:xyz" with maxDepth=3 increments
// "log", "log:error", and "log:error:auth" (3 colons → 4 parts → 3 prefix levels).
// But "wlog:uuid" (1 colon → 2 parts) only increments "wlog".
func (s *BadgerStore) TreeGroupCounts(maxDepth int) (map[string]int, error) {
	counts := make(map[string]int)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			parts := strings.SplitN(key, ":", maxDepth+1)
			// Stop before the last part — the last segment is the leaf, not a group.
			limit := len(parts) - 1
			if limit > maxDepth {
				limit = maxDepth
			}
			prefix := ""
			for i := 0; i < limit; i++ {
				if i > 0 {
					prefix += ":"
				}
				prefix += parts[i]
				counts[prefix]++
			}
		}
		return nil
	})
	return counts, err
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

// equalFold is adapted from strings.EqualFold.
func equalFold(tr, sr rune) bool {
	if tr == sr {
		return true
	}
	if tr < sr {
		tr, sr = sr, tr
	}
	// Fast-path ASCII.
	if tr < utf8.RuneSelf {
		// Normalize ASCII case by comparing lower/upper pairs.
		if 'A' <= sr && sr <= 'Z' && tr == sr+'a'-'A' {
			return true
		}
		return false
	}

	// Fall back to SimpleFold for the general case, which cycles equivalents.
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
