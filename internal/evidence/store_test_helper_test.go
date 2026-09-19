package evidence

import (
	"path/filepath"
	"testing"
)

// NewTestStore 在临时目录中创建一个快照文件支持的 Store。
func NewTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := NewStore(filepath.Join(t.TempDir(), "evidence.json"))
	if err != nil {
		t.Fatalf("创建测试存储失败: %v", err)
	}
	return st
}
