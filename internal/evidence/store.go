package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// snapshot 是持久化文件的完整结构。
type snapshot struct {
	Systems      map[string]SystemContext        `json:"systems"`
	Institutions map[string]Institution          `json:"institutions"`
	Frameworks   map[string]Framework            `json:"frameworks"`
	Courses      map[string]CourseVersion        `json:"courses"`
	CPDs         map[string]CPDProgram           `json:"cpds"`
	Sources      map[string]Source               `json:"sources"`
	Mappings     map[string]*Mapping             `json:"mappings"`
	Recs         map[string]PolicyRecommendation `json:"recs"`
	Calibrations map[string]*Calibration         `json:"calibrations"`
	Pilots       map[string]*Pilot               `json:"pilots"`
	Adjustments  map[string]CourseAdjustment     `json:"adjustments"`
}

// Store 以内存结构保管数据，并在每次变更后落盘为 JSON 快照。
type Store struct {
	mu   sync.RWMutex
	path string
	snapshot
}

// NewStore 打开（或创建）path 指向的快照文件。
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	s.init()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.snapshot); err != nil {
		return nil, err
	}
	s.ensureMaps()
	return s, nil
}

func (s *Store) init() {
	s.snapshot = snapshot{
		Systems:      map[string]SystemContext{},
		Institutions: map[string]Institution{},
		Frameworks:   map[string]Framework{},
		Courses:      map[string]CourseVersion{},
		CPDs:         map[string]CPDProgram{},
		Sources:      map[string]Source{},
		Mappings:     map[string]*Mapping{},
		Recs:         map[string]PolicyRecommendation{},
		Calibrations: map[string]*Calibration{},
		Pilots:       map[string]*Pilot{},
		Adjustments:  map[string]CourseAdjustment{},
	}
}

func (s *Store) ensureMaps() {
	if s.Systems == nil {
		s.init()
	}
}

// persist 必须在持有写锁时调用。
func (s *Store) persist() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.snapshot, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) rlock() func() {
	s.mu.RLock()
	return s.mu.RUnlock
}

func (s *Store) lock() func() {
	s.mu.Lock()
	return s.mu.Unlock
}
