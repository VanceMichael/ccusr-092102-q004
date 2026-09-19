package evidence

import (
	"fmt"
	"sort"
	"time"
)

// 映射对象类型。
const (
	RefFrameworkDomain = "framework_domain"
	RefCourse          = "course"
	RefCPD             = "cpd"
)

// 映射的概念类别。
const (
	KindCompetency = "competency"
	KindCourse     = "course"
	KindPractice   = "practice"
	KindCPD        = "cpd"
)

// SubmitMappingInput 是专家提交跨体系映射的输入。
type SubmitMappingInput struct {
	LogicalID     string // 可空；为空时自动分配。修订时沿用
	Kind          string
	Source        MappingRef
	Target        MappingRef
	Confidence    float64
	Applicability string
	Rationale     string
	ExpertID      string
}

func (s *Service) validateRef(ref MappingRef) error {
	if ref.SystemID == "" || ref.Kind == "" || ref.RefID == "" {
		return fmt.Errorf("%w: 映射引用必须包含 system_id/kind/ref_id", ErrInvalid)
	}
	if _, ok := s.store.Systems[ref.SystemID]; !ok {
		return fmt.Errorf("%w: 制度背景 %s 不存在", ErrInvalid, ref.SystemID)
	}
	switch ref.Kind {
	case RefFrameworkDomain:
		// RefID 形如 "框架ID#领域key"
		fwID, domainKey, ok := splitRef(ref.RefID)
		if !ok {
			return fmt.Errorf("%w: 素养领域引用格式应为 框架ID#领域key", ErrInvalid)
		}
		fw, ok := s.store.Frameworks[fwID]
		if !ok {
			return fmt.Errorf("%w: 框架 %s 不存在", ErrInvalid, fwID)
		}
		if fw.SystemID != ref.SystemID {
			return fmt.Errorf("%w: 框架 %s 不属于制度 %s", ErrInvalid, fwID, ref.SystemID)
		}
		found := false
		for _, d := range fw.Domains {
			if d.Key == domainKey {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: 框架 %s 中不存在领域 %s", ErrInvalid, fwID, domainKey)
		}
	case RefCourse:
		cv, ok := s.store.Courses[ref.RefID]
		if !ok {
			return fmt.Errorf("%w: 课程版本 %s 不存在", ErrInvalid, ref.RefID)
		}
		if cv.SystemID != ref.SystemID {
			return fmt.Errorf("%w: 课程 %s 不属于制度 %s", ErrInvalid, ref.RefID, ref.SystemID)
		}
	case RefCPD:
		p, ok := s.store.CPDs[ref.RefID]
		if !ok {
			return fmt.Errorf("%w: CPD 项目 %s 不存在", ErrInvalid, ref.RefID)
		}
		if p.SystemID != ref.SystemID {
			return fmt.Errorf("%w: CPD %s 不属于制度 %s", ErrInvalid, ref.RefID, ref.SystemID)
		}
	default:
		return fmt.Errorf("%w: 未知引用类型 %s", ErrInvalid, ref.Kind)
	}
	return nil
}

func splitRef(v string) (string, string, bool) {
	for i := 0; i < len(v); i++ {
		if v[i] == '#' {
			return v[:i], v[i+1:], true
		}
	}
	return "", "", false
}

// validateMappingKinds 保证同类对象才互相映射：素养对素养、课程对课程、
// 实践环节（挂在课程版本上）对实践环节、CPD 对 CPD，禁止把职前课程当成 CPD。
func validateMappingKinds(conceptKind, sourceKind, targetKind string) error {
	want := map[string]string{
		KindCompetency: RefFrameworkDomain,
		KindCourse:     RefCourse,
		KindPractice:   RefCourse,
		KindCPD:        RefCPD,
	}[conceptKind]
	if want == "" {
		return fmt.Errorf("%w: 未知映射类别 %s", ErrInvalid, conceptKind)
	}
	if sourceKind != want || targetKind != want {
		return fmt.Errorf("%w: %s 类映射两端都必须是 %s 类型，收到 %s 与 %s",
			ErrInvalid, conceptKind, want, sourceKind, targetKind)
	}
	return nil
}

// validateMappingInput 校验字段取值、引用真实性、跨制度与同类对象约束。必须持写锁调用。
func (s *Service) validateMappingInput(in SubmitMappingInput) error {
	if in.Kind != KindCompetency && in.Kind != KindCourse && in.Kind != KindPractice && in.Kind != KindCPD {
		return fmt.Errorf("%w: 未知映射类别 %s", ErrInvalid, in.Kind)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return fmt.Errorf("%w: 置信度必须在 0 到 1 之间", ErrInvalid)
	}
	if in.Applicability == "" {
		return fmt.Errorf("%w: 适用范围不能为空", ErrInvalid)
	}
	if in.ExpertID == "" {
		return fmt.Errorf("%w: 专家标识不能为空", ErrInvalid)
	}
	if err := s.validateRef(in.Source); err != nil {
		return err
	}
	if err := s.validateRef(in.Target); err != nil {
		return err
	}
	if in.Source.SystemID == in.Target.SystemID {
		return ErrSystemMismatch
	}
	return validateMappingKinds(in.Kind, in.Source.Kind, in.Target.Kind)
}

// submitLocked 必须在持有写锁时调用；logicalID 为空时自动分配。
func (s *Service) submitLocked(in SubmitMappingInput, logicalID string) (string, error) {
	if err := s.validateMappingInput(in); err != nil {
		return "", err
	}
	logical := logicalID
	if logical == "" {
		logical = fmt.Sprintf("M-%d", len(s.store.Mappings)+1)
	}
	return s.insertMapping(in, logical)
}

// SubmitMapping 登记一条新的待审映射，返回其实例 ID（"逻辑ID@版本"）。
func (s *Service) SubmitMapping(in SubmitMappingInput) (string, error) {
	unlock := s.store.lock()
	defer unlock()
	return s.submitLocked(in, in.LogicalID)
}

// insertMapping 在同一逻辑 ID 下追加新版本，必须在持有写锁时调用。
func (s *Service) insertMapping(in SubmitMappingInput, logical string) (string, error) {
	version := 0
	for _, m := range s.store.Mappings {
		if m.LogicalID == logical && m.Version > version {
			version = m.Version
		}
	}
	version++
	m := &Mapping{
		ID:            fmt.Sprintf("%s@%d", logical, version),
		LogicalID:     logical,
		Version:       version,
		Kind:          in.Kind,
		Source:        in.Source,
		Target:        in.Target,
		Confidence:    in.Confidence,
		Applicability: in.Applicability,
		Rationale:     in.Rationale,
		ExpertID:      in.ExpertID,
		Status:        MappingProposed,
		CreatedAt:     s.now(),
	}
	if version > 1 {
		m.Supersedes = fmt.Sprintf("%s@%d", logical, version-1)
	}
	s.store.Mappings[m.ID] = m
	return m.ID, s.store.persist()
}

// ReviseMapping 基于某条既有映射提交修订版；旧版标记为被替代，新版须重新审批。
// 留空的字段（置信度传 0 时仍按 0 处理——需显式传值）沿用旧版。
func (s *Service) ReviseMapping(baseInstanceID, expertID string, in SubmitMappingInput) (string, error) {
	unlock := s.store.lock()
	defer unlock()
	base, ok := s.store.Mappings[baseInstanceID]
	if !ok {
		return "", fmt.Errorf("%w: 映射 %s", ErrNotFound, baseInstanceID)
	}
	if base.Status == MappingProposed {
		return "", fmt.Errorf("%w: 待审映射 %s 不能直接修订，等待审批结论", ErrMappingNotPending, baseInstanceID)
	}
	if in.Confidence == 0 {
		in.Confidence = base.Confidence
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return "", fmt.Errorf("%w: 置信度必须在 0 到 1 之间", ErrInvalid)
	}
	if in.Source.RefID == "" {
		in.Source = base.Source
	}
	if in.Target.RefID == "" {
		in.Target = base.Target
	}
	if in.Applicability == "" {
		in.Applicability = base.Applicability
	}
	if in.ExpertID == "" {
		in.ExpertID = expertID
	}
	if in.Kind == "" {
		in.Kind = base.Kind
	}
	id, err := s.submitLocked(in, base.LogicalID)
	if err != nil {
		return "", err
	}
	base.Status = MappingSuperseded
	return id, s.store.persist()
}

// AddObjection 对映射登记异议（待审或已获批均可，获批映射的异议会在比较中标记争议）。
func (s *Service) AddObjection(instanceID, expertID, reason string) error {
	if expertID == "" || reason == "" {
		return fmt.Errorf("%w: 异议必须包含专家标识与理由", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()
	m, ok := s.store.Mappings[instanceID]
	if !ok {
		return fmt.Errorf("%w: 映射 %s", ErrNotFound, instanceID)
	}
	m.Objections = append(m.Objections, Objection{ExpertID: expertID, Reason: reason, CreatedAt: s.now()})
	return s.store.persist()
}

// ReviewMapping 审批映射。驳回必须给出书面理由。
func (s *Service) ReviewMapping(instanceID, reviewerID, decision, comment string) error {
	if reviewerID == "" {
		return fmt.Errorf("%w: 审批人标识不能为空", ErrInvalid)
	}
	if decision != MappingApproved && decision != MappingRejected {
		return fmt.Errorf("%w: 审批结论必须是 approved 或 rejected", ErrInvalid)
	}
	if decision == MappingRejected && comment == "" {
		return ErrObjectionMissing
	}
	unlock := s.store.lock()
	defer unlock()
	m, ok := s.store.Mappings[instanceID]
	if !ok {
		return fmt.Errorf("%w: 映射 %s", ErrNotFound, instanceID)
	}
	if m.Status != MappingProposed {
		return fmt.Errorf("%w: %s 当前状态为 %s", ErrMappingNotPending, instanceID, m.Status)
	}
	now := s.now()
	m.Status = decision
	m.DecidedAt = &now
	m.Reviews = append(m.Reviews, ReviewRecord{ReviewerID: reviewerID, Decision: decision, Comment: comment, At: now})
	if decision == MappingRejected {
		m.Objections = append(m.Objections, Objection{ExpertID: reviewerID, Reason: comment, CreatedAt: now})
	}
	return s.store.persist()
}

// GetMapping 返回映射实例。
func (s *Service) GetMapping(instanceID string) (Mapping, error) {
	unlock := s.store.rlock()
	defer unlock()
	m, ok := s.store.Mappings[instanceID]
	if !ok {
		return Mapping{}, fmt.Errorf("%w: 映射 %s", ErrNotFound, instanceID)
	}
	return *m, nil
}

// MappingLineage 返回同一逻辑 ID 下的全部版本，按版本号排序。
func (s *Service) MappingLineage(logicalID string) ([]Mapping, error) {
	unlock := s.store.rlock()
	defer unlock()
	var out []Mapping
	for _, m := range s.store.Mappings {
		if m.LogicalID == logicalID {
			out = append(out, *m)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: 逻辑映射 %s", ErrNotFound, logicalID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// ComparisonPair 是正式比较中的一对可比对象。
type ComparisonPair struct {
	MappingID     string     `json:"mapping_id"`
	Kind          string     `json:"kind"`
	Source        MappingRef `json:"source"`
	Target        MappingRef `json:"target"`
	Confidence    float64    `json:"confidence"`
	Applicability string     `json:"applicability"`
	Rationale     string     `json:"rationale"`
	Disputed      bool       `json:"disputed"` // 已获批但存在未撤回异议
	ApprovedAt    time.Time  `json:"approved_at"`
}

// ComparisonView 是某一类概念的正式可比视图。
type ComparisonView struct {
	Kind            string           `json:"kind"`
	ComparablePairs []ComparisonPair `json:"comparable_pairs"`
	PendingCount    int              `json:"pending_count"`
	RejectedCount   int              `json:"rejected_count"`
	// UncoveredSystems 是尚无任何获批映射涉及的制度，提示"尚无共识"的范围。
	UncoveredSystems []string `json:"uncovered_systems"`
}

// Compare 只汇集状态为 approved 且未被替代的映射；待审/驳回/被替代映射一律不进入。
func (s *Service) Compare(kind string) (ComparisonView, error) {
	switch kind {
	case KindCompetency, KindCourse, KindPractice, KindCPD:
	default:
		return ComparisonView{}, fmt.Errorf("%w: 未知映射类别 %s", ErrInvalid, kind)
	}
	unlock := s.store.rlock()
	defer unlock()
	view := ComparisonView{Kind: kind, ComparablePairs: []ComparisonPair{}}
	covered := map[string]bool{}
	for _, m := range s.store.Mappings {
		if m.Kind != kind {
			continue
		}
		switch m.Status {
		case MappingProposed:
			view.PendingCount++
		case MappingRejected:
			view.RejectedCount++
		case MappingApproved:
			view.ComparablePairs = append(view.ComparablePairs, ComparisonPair{
				MappingID:     m.ID,
				Kind:          m.Kind,
				Source:        m.Source,
				Target:        m.Target,
				Confidence:    m.Confidence,
				Applicability: m.Applicability,
				Rationale:     m.Rationale,
				Disputed:      len(m.Objections) > 0,
				ApprovedAt:    *m.DecidedAt,
			})
			covered[m.Source.SystemID] = true
			covered[m.Target.SystemID] = true
		}
	}
	sort.Slice(view.ComparablePairs, func(i, j int) bool {
		return view.ComparablePairs[i].MappingID < view.ComparablePairs[j].MappingID
	})
	for id := range s.store.Systems {
		if !covered[id] {
			view.UncoveredSystems = append(view.UncoveredSystems, id)
		}
	}
	sort.Strings(view.UncoveredSystems)
	return view, nil
}
