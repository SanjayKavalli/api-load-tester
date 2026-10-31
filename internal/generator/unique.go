package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofrs/flock"
)

var ErrUniqueUnavailable = errors.New("cannot generate an unused value after 128 attempts; expand the pattern or use run_id and sequence")
var ErrUniqueGeneration = errors.New("unique field generation failed")

type UniqueStore struct {
	mu   sync.Mutex
	seen map[string]bool
	file *os.File
	lock *flock.Flock
}

type Session struct {
	RunID         string
	ReuseExisting bool
	store         *UniqueStore
	counter       atomic.Uint64
}

func DefaultHistoryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "load-lab", "unique-values.jsonl"), nil
}

func OpenUniqueStore(path string) (*UniqueStore, error) {
	store := &UniqueStore{seen: map[string]bool{}}
	if path == "" {
		return store, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	store.lock = flock.New(path + ".lock")
	locked, err := store.lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, fmt.Errorf("unique history is in use by another runner")
	}
	success := false
	defer func() {
		if !success {
			store.Close()
		}
	}()
	store.file, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(store.file)
	for {
		var digest string
		err := decoder.Decode(&digest)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unique history is damaged; restore it before running: %w", err)
		}
		if len(digest) != 64 {
			return nil, fmt.Errorf("invalid unique history entry")
		}
		store.seen[digest] = true
	}
	success = true
	return store, nil
}

func (store *UniqueStore) Close() error {
	if store.file != nil {
		store.file.Close()
	}
	if store.lock != nil {
		return store.lock.Unlock()
	}
	return nil
}

func (store *UniqueStore) reserve(path []string, value any) (bool, error) {
	switch scalar := value.(type) {
	case string:
		if strings.Contains(scalar, "{{") {
			return false, fmt.Errorf("Unique fields must use fully generated rule values, not placeholders")
		}
	case float64, int, int64, uint64, bool:
	default:
		return false, fmt.Errorf("Unique supports string, number, or boolean rule results only")
	}
	encoded, err := json.Marshal([]any{path, value})
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(encoded)
	key := hex.EncodeToString(digest[:])
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.seen[key] {
		return false, nil
	}
	if store.file != nil {
		if err := json.NewEncoder(store.file).Encode(key); err != nil {
			return false, err
		}
		if err := store.file.Sync(); err != nil {
			return false, err
		}
	}
	store.seen[key] = true
	return true, nil
}

func NewSession(store *UniqueStore, reuseExisting bool) (*Session, error) {
	runID, err := newUUID()
	if err != nil {
		return nil, err
	}
	if store == nil {
		store, _ = OpenUniqueStore("")
	}
	return &Session{RunID: runID, ReuseExisting: reuseExisting, store: store}, nil
}
