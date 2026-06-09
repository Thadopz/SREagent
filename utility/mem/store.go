package mem

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

const memoryDirEnv = "SUPERBIZ_MEMORY_DIR"

type memoryStore interface {
	Load(id string) (*memorySnapshot, error)
	Save(snapshot *memorySnapshot) error
}

type appendMemoryStore interface {
	Append(snapshot *memorySnapshot, msg *schema.Message) error
}

type memorySnapshot struct {
	ID            string            `json:"id"`
	UserID        string            `json:"user_id"`
	Messages      []*schema.Message `json:"messages"`
	Summary       string            `json:"summary"`
	MaxWindowSize int               `json:"max_window_size"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type fileMemoryStore struct {
	dir string
}

func newFileMemoryStore(dir string) *fileMemoryStore {
	return &fileMemoryStore{dir: dir}
}

func defaultMemoryDir() string {
	if dir := strings.TrimSpace(os.Getenv(memoryDirEnv)); dir != "" {
		return dir
	}
	return filepath.Join("data", "memory")
}

func (s *fileMemoryStore) Load(id string) (*memorySnapshot, error) {
	body, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var snapshot memorySnapshot
	if err = json.Unmarshal(body, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.ID == "" {
		snapshot.ID = id
	}
	return &snapshot, nil
}

func (s *fileMemoryStore) Save(snapshot *memorySnapshot) error {
	if snapshot == nil || strings.TrimSpace(snapshot.ID) == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(snapshot.ID), body, 0o600)
}

func (s *fileMemoryStore) path(id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}
