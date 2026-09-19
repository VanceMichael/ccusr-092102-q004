package evidence

import (
	"fmt"
	"sort"
)

// CreateRecommendationInput 用于创建政策建议。
type CreateRecommendationInput struct {
	ID    string
	Title string
	// MappingIDs 必须全部指向"获批且未被替代"的映射；不满足时拒绝创建。
	MappingIDs []string
	PilotID    string
	Notes      string
}

// CreateRecommendation 创建政策建议，并冻结当时获批映射的完整快照。
// 之后映射即便被修订/替代，建议仍可回溯其依据的旧版内容。
func (s *Service) CreateRecommendation(in CreateRecommendationInput) (PolicyRecommendation, error) {
	if in.ID == "" || in.Title == "" {
		return PolicyRecommendation{}, fmt.Errorf("%w: 建议 id 与标题不能为空", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()
	if _, exists := s.store.Recs[in.ID]; exists {
		return PolicyRecommendation{}, fmt.Errorf("%w: 建议 %s", ErrAlreadyExists, in.ID)
	}
	if len(in.MappingIDs) == 0 {
		return PolicyRecommendation{}, fmt.Errorf("%w: 政策建议至少需要一条获批映射作为证据", ErrInvalid)
	}
	basis := EvidenceBasis{
		MappingSnapshot: make([]Mapping, 0, len(in.MappingIDs)),
		CourseRefs:      []string{},
		SystemIDs:       []string{},
		PilotID:         in.PilotID,
		Notes:           in.Notes,
	}
	systems := map[string]bool{}
	for _, mid := range in.MappingIDs {
		m, ok := s.store.Mappings[mid]
		if !ok {
			return PolicyRecommendation{}, fmt.Errorf("%w: 映射 %s", ErrNotFound, mid)
		}
		if m.Status != MappingApproved {
			return PolicyRecommendation{}, fmt.Errorf("%w: %s 状态为 %s，不能作为正式建议依据", ErrMappingNotApproved, mid, m.Status)
		}
		basis.MappingSnapshot = append(basis.MappingSnapshot, *m)
		systems[m.Source.SystemID] = true
		systems[m.Target.SystemID] = true
		basis.CourseRefs = append(basis.CourseRefs, courseRefsOf(*m)...)
	}
	if in.PilotID != "" {
		pilot, ok := s.store.Pilots[in.PilotID]
		if !ok {
			return PolicyRecommendation{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, in.PilotID)
		}
		basis.CalibrationID = pilot.CalibrationID
		for _, ref := range pilot.CalibrationCurriculumRefs(s.store) {
			basis.CourseRefs = append(basis.CourseRefs, ref)
		}
	}
	for sys := range systems {
		basis.SystemIDs = append(basis.SystemIDs, sys)
	}
	sort.Strings(basis.SystemIDs)
	sort.Strings(basis.CourseRefs)
	basis.CourseRefs = dedup(basis.CourseRefs)
	rec := PolicyRecommendation{
		ID:        in.ID,
		Title:     in.Title,
		Basis:     basis,
		CreatedAt: s.now(),
	}
	s.store.Recs[in.ID] = rec
	return rec, s.store.persist()
}

func courseRefsOf(m Mapping) []string {
	var refs []string
	if m.Source.Kind == RefCourse {
		refs = append(refs, m.Source.RefID)
	}
	if m.Target.Kind == RefCourse {
		refs = append(refs, m.Target.RefID)
	}
	return refs
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// GetRecommendation 返回政策建议及其冻结的证据快照。
func (s *Service) GetRecommendation(id string) (PolicyRecommendation, error) {
	unlock := s.store.rlock()
	defer unlock()
	rec, ok := s.store.Recs[id]
	if !ok {
		return PolicyRecommendation{}, fmt.Errorf("%w: 建议 %s", ErrNotFound, id)
	}
	return rec, nil
}

// BackedRecommendations 返回依据中包含指定映射实例（含旧版本）的全部政策建议。
// 即使映射后来被修订为 v2，仍能查出 v1 曾支撑过哪些建议。
func (s *Service) BackedRecommendations(mappingInstanceID string) []PolicyRecommendation {
	unlock := s.store.rlock()
	defer unlock()
	var out []PolicyRecommendation
	for _, rec := range s.store.Recs {
		for _, m := range rec.Basis.MappingSnapshot {
			if m.ID == mappingInstanceID {
				out = append(out, rec)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RecommendationFreshness 描述建议证据快照与当前映射状态的差异。
type RecommendationFreshness struct {
	RecommendationID string              `json:"recommendation_id"`
	Entries          []SnapshotFreshness `json:"entries"`
	AllCurrent       bool                `json:"all_current"`
}

// SnapshotFreshness 单条快照映射的现况。
type SnapshotFreshness struct {
	MappingID       string `json:"mapping_id"`
	CurrentStatus   string `json:"current_status"`
	LatestVersion   int    `json:"latest_version"`
	SnapshotVersion int    `json:"snapshot_version"`
	Changed         bool   `json:"changed"`
}

// CheckRecommendationFreshness 检查建议依据的映射是否已被修订或状态改变。
func (s *Service) CheckRecommendationFreshness(recID string) (RecommendationFreshness, error) {
	unlock := s.store.rlock()
	defer unlock()
	rec, ok := s.store.Recs[recID]
	if !ok {
		return RecommendationFreshness{}, fmt.Errorf("%w: 建议 %s", ErrNotFound, recID)
	}
	out := RecommendationFreshness{RecommendationID: recID, AllCurrent: true, Entries: []SnapshotFreshness{}}
	for _, snap := range rec.Basis.MappingSnapshot {
		latest := 0
		status := ""
		for _, m := range s.store.Mappings {
			if m.LogicalID == snap.LogicalID && m.Version > latest {
				latest = m.Version
				status = m.Status
			}
		}
		entry := SnapshotFreshness{
			MappingID:       snap.ID,
			CurrentStatus:   status,
			LatestVersion:   latest,
			SnapshotVersion: snap.Version,
			Changed:         latest != snap.Version || status != snap.Status,
		}
		if entry.Changed {
			out.AllCurrent = false
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}
