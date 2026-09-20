package curriculum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Service 是证据后端的内存实现。所有写入都受互斥保护；
// 领域规则集中在此处，HTTP 层只负责编解码。
type Service struct {
	mu sync.RWMutex

	countries    map[string]Country
	institutions map[string]Institution
	frameworks   map[string]CompetencyFramework
	courses      map[string]Course
	mappings     map[string]Mapping
	calibers     map[string]CaliberVersion
	pilots       map[string]Pilot
	stages       map[string]PilotStage // stage.ID -> stage
	pilotStages  map[string][]string   // pilotID -> 有序 stage ID
	recs         map[string]PolicyRecommendation
	proposals    map[string]PolicyProposal

	now func() time.Time
}

// NewService 创建空服务。
func NewService() *Service {
	return &Service{
		countries:    map[string]Country{},
		institutions: map[string]Institution{},
		frameworks:   map[string]CompetencyFramework{},
		courses:      map[string]Course{},
		mappings:     map[string]Mapping{},
		calibers:     map[string]CaliberVersion{},
		pilots:       map[string]Pilot{},
		stages:       map[string]PilotStage{},
		pilotStages:  map[string][]string{},
		recs:         map[string]PolicyRecommendation{},
		proposals:    map[string]PolicyProposal{},
		now:          func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) stamp() string { return s.now().Format(time.RFC3339) }

// ---------- 制度背景与原始材料 ----------

// RegisterCountry 登记国家/地区的制度背景。
func (s *Service) RegisterCountry(c Country) error {
	if c.Code == "" || c.Name == "" {
		return fmt.Errorf("%w: 国家代码与名称必填", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.countries[c.Code]; ok {
		return fmt.Errorf("%w: 国家 %s 已登记", ErrConflict, c.Code)
	}
	s.countries[c.Code] = c
	return nil
}

// RegisterInstitution 登记院校。
func (s *Service) RegisterInstitution(i Institution) error {
	if i.ID == "" || i.Name == "" || i.CountryCode == "" {
		return fmt.Errorf("%w: 院校 ID、名称与国家代码必填", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.countries[i.CountryCode]; !ok {
		return fmt.Errorf("%w: 国家 %s 未登记", ErrInvalid, i.CountryCode)
	}
	if _, ok := s.institutions[i.ID]; ok {
		return fmt.Errorf("%w: 院校 %s 已登记", ErrConflict, i.ID)
	}
	s.institutions[i.ID] = i
	return nil
}

// RecordFramework 登记能力框架的一个不可变版本。
func (s *Service) RecordFramework(f CompetencyFramework) error {
	if f.ID == "" || f.CountryCode == "" || f.Version == "" || len(f.Items) == 0 {
		return fmt.Errorf("%w: 框架 ID、国家、版本与能力条目必填", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.countries[f.CountryCode]; !ok {
		return fmt.Errorf("%w: 国家 %s 未登记", ErrInvalid, f.CountryCode)
	}
	if f.InstitutionID != "" {
		if _, ok := s.institutions[f.InstitutionID]; !ok {
			return fmt.Errorf("%w: 院校 %s 未登记", ErrInvalid, f.InstitutionID)
		}
	}
	if _, ok := s.frameworks[f.ID]; ok {
		return fmt.Errorf("%w: 框架版本 %s 已存在，修订须登记新版本", ErrConflict, f.ID)
	}
	seen := map[string]bool{}
	for _, it := range f.Items {
		if it.Code == "" || it.Description == "" {
			return fmt.Errorf("%w: 能力条目代码与原始定义不能为空", ErrInvalid)
		}
		if seen[it.Code] {
			return fmt.Errorf("%w: 能力条目代码 %s 重复", ErrInvalid, it.Code)
		}
		seen[it.Code] = true
	}
	s.frameworks[f.ID] = f
	return nil
}

// RecordCourse 登记某培养方案版本中的原始课程。记录不可变：
// 同一课程 ID 重复登记即冲突，方案修订须使用新的 program_version 与新 ID。
func (s *Service) RecordCourse(c Course) error {
	if c.ID == "" || c.InstitutionID == "" || c.ProgramVersion == "" || c.LocalCode == "" || c.NameLocal == "" {
		return fmt.Errorf("%w: 课程 ID、院校、方案版本、原始代码与原文名称必填", ErrInvalid)
	}
	if c.CreditSystem == "" {
		return fmt.Errorf("%w: 必须保留原始学分制（credit_system）", ErrInvalid)
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("%w: 课程必须附研究/官方出处", ErrInvalid)
	}
	for t, mode := range c.DeliveryByTopic {
		if mode != DeliveryStandalone && mode != DeliveryEmbedded {
			return fmt.Errorf("%w: 主题 %s 的开设方式只能是 standalone 或 embedded", ErrInvalid, t)
		}
		if !contains(c.TopicTags, t) {
			return fmt.Errorf("%w: 开设方式主题 %s 未出现在 topic_tags", ErrInvalid, t)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.institutions[c.InstitutionID]
	if !ok {
		return fmt.Errorf("%w: 院校 %s 未登记", ErrInvalid, c.InstitutionID)
	}
	if _, ok := s.courses[c.ID]; ok {
		return fmt.Errorf("%w: 课程 %s 已登记，原始记录不可修改", ErrConflict, c.ID)
	}
	for _, ref := range c.CompetencyRefs {
		parts := strings.SplitN(ref, "#", 2)
		if len(parts) != 2 {
			return fmt.Errorf("%w: 能力引用 %s 格式应为 框架ID#条目代码", ErrInvalid, ref)
		}
		fw, ok := s.frameworks[parts[0]]
		if !ok {
			return fmt.Errorf("%w: 能力引用 %s 的框架不存在", ErrInvalid, ref)
		}
		if fw.CountryCode != inst.CountryCode {
			return fmt.Errorf("%w: 课程 %s 引用了他国框架 %s", ErrInvalid, c.ID, ref)
		}
		if !findItem(fw, parts[1]) {
			return fmt.Errorf("%w: 框架 %s 中不存在条目 %s", ErrInvalid, parts[0], parts[1])
		}
	}
	if c.RecordedOn == "" {
		c.RecordedOn = s.stamp()
	}
	s.courses[c.ID] = c
	return nil
}

// ---------- 跨体系映射 ----------

// MappingInput 是提交映射意见所需的字段。
type MappingInput struct {
	Concept      string        `json:"concept"`
	Anchor       ConceptAnchor `json:"anchor"`
	Relationship string        `json:"relationship"`
	Expert       string        `json:"expert"`
	Confidence   float64       `json:"confidence"`
	Basis        string        `json:"basis"`
	Scope        MappingScope  `json:"scope"`
}

func (in MappingInput) validate() error {
	if in.Concept == "" || in.Expert == "" || in.Basis == "" {
		return fmt.Errorf("%w: 规范概念、专家与判定依据必填", ErrInvalid)
	}
	if in.Anchor.CountryCode == "" || in.Anchor.Kind == "" || in.Anchor.Ref == "" {
		return fmt.Errorf("%w: 映射锚点必须指明国家、类型与本地引用", ErrInvalid)
	}
	switch in.Anchor.Kind {
	case "competency", "course_topic", "practice", "cpd":
	default:
		return fmt.Errorf("%w: 锚点类型 %s 不支持", ErrInvalid, in.Anchor.Kind)
	}
	switch in.Relationship {
	case "equivalent", "narrower", "broader", "related":
	default:
		return fmt.Errorf("%w: 关系必须是 equivalent/narrower/broader/related", ErrInvalid)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return fmt.Errorf("%w: 置信度必须在 0~1 之间", ErrInvalid)
	}
	return nil
}

// SubmitMapping 提交一条映射意见。新意见始终为 submitted，
// 未经批准不会进入任何口径或正式比较。
func (s *Service) SubmitMapping(in MappingInput) (Mapping, error) {
	if err := in.validate(); err != nil {
		return Mapping{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.countries[in.Anchor.CountryCode]; !ok {
		return Mapping{}, fmt.Errorf("%w: 国家 %s 未登记", ErrInvalid, in.Anchor.CountryCode)
	}
	m := Mapping{
		ID:           fmt.Sprintf("M%03d", len(s.mappings)+1),
		Concept:      in.Concept,
		Anchor:       in.Anchor,
		Relationship: in.Relationship,
		Expert:       in.Expert,
		Confidence:   in.Confidence,
		Basis:        in.Basis,
		Scope:        in.Scope,
		Status:       MappingSubmitted,
		SubmittedOn:  s.stamp(),
	}
	s.mappings[m.ID] = m
	return m, nil
}

// MappingReview 是审批决定。
type MappingReview struct {
	Reviewer string `json:"reviewer"`
	Approve  bool   `json:"approve"`
	Note     string `json:"note,omitempty"`
}

// ReviewMapping 批准或驳回一条映射。
func (s *Service) ReviewMapping(id string, r MappingReview) (Mapping, error) {
	if r.Reviewer == "" {
		return Mapping{}, fmt.Errorf("%w: 审批人必填", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.mappings[id]
	if !ok {
		return Mapping{}, fmt.Errorf("%w: 映射 %s", ErrNotFound, id)
	}
	if m.Status != MappingSubmitted {
		return Mapping{}, fmt.Errorf("%w: 仅待审批映射可审批，当前状态 %s", ErrState, m.Status)
	}
	m.ReviewedBy = r.Reviewer
	m.ReviewedOn = s.stamp()
	m.ReviewNote = r.Note
	if r.Approve {
		m.Status = MappingApproved
	} else {
		m.Status = MappingRejected
	}
	s.mappings[id] = m
	return m, nil
}

// AddObjection 对已存在的映射登记专家异议。
func (s *Service) AddObjection(id string, o Objection) (Mapping, error) {
	if o.Expert == "" || o.Reason == "" {
		return Mapping{}, fmt.Errorf("%w: 异议专家与理由必填", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.mappings[id]
	if !ok {
		return Mapping{}, fmt.Errorf("%w: 映射 %s", ErrNotFound, id)
	}
	if m.Status == MappingRejected || m.Status == MappingSuperseded {
		return Mapping{}, fmt.Errorf("%w: 已驳回或已替代的映射不再接受异议", ErrState)
	}
	if o.On == "" {
		o.On = s.stamp()
	}
	m.Objections = append(m.Objections, o)
	s.mappings[id] = m
	return m, nil
}

// ReviseMapping 用新意见修订映射：旧版标记为 superseded 并永久保留，
// 新版本从 submitted 重新走审批；修订链通过 revision_of 维系。
func (s *Service) ReviseMapping(oldID string, in MappingInput) (Mapping, error) {
	if err := in.validate(); err != nil {
		return Mapping{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.mappings[oldID]
	if !ok {
		return Mapping{}, fmt.Errorf("%w: 映射 %s", ErrNotFound, oldID)
	}
	if old.Status == MappingSuperseded || old.Status == MappingRejected {
		return Mapping{}, fmt.Errorf("%w: 旧版状态 %s 不可再修订", ErrState, old.Status)
	}
	if in.Concept != old.Concept {
		return Mapping{}, fmt.Errorf("%w: 修订不得改变规范概念，概念变化应提交新映射", ErrInvalid)
	}
	old.Status = MappingSuperseded
	s.mappings[oldID] = old
	m := Mapping{
		ID:           fmt.Sprintf("M%03d", len(s.mappings)+1),
		RevisionOf:   oldID,
		Concept:      in.Concept,
		Anchor:       in.Anchor,
		Relationship: in.Relationship,
		Expert:       in.Expert,
		Confidence:   in.Confidence,
		Basis:        in.Basis,
		Scope:        in.Scope,
		Status:       MappingSubmitted,
		SubmittedOn:  s.stamp(),
	}
	s.mappings[m.ID] = m
	return m, nil
}

// ---------- 正式比较 ----------

// DeliveryEvidence 是“独立设课/融入课程”的原始证据片段。
type DeliveryEvidence struct {
	CourseID       string          `json:"course_id"`
	InstitutionID  string          `json:"institution_id"`
	ProgramVersion string          `json:"program_version"`
	Mode           DeliveryMode    `json:"mode"`
	Source         SourceReference `json:"source"`
}

// CountryConceptEvidence 是一国在某规范概念上的可比证据。
type CountryConceptEvidence struct {
	CountryCode string `json:"country_code"`
	MappingID   string `json:"mapping_id"`
	// MappingStatus 该映射版本的当前状态；口径证据链中它可能已被修订替代，
	// 但作为当时冻结的依据仍保留在比较中。
	MappingStatus        string             `json:"mapping_status"`
	Relationship         string             `json:"relationship"`
	Confidence           float64            `json:"confidence"`
	Scope                MappingScope       `json:"scope"`
	UnresolvedObjections []Objection        `json:"unresolved_objections,omitempty"`
	DeliveryModes        []string           `json:"delivery_modes,omitempty"`
	DeliveryEvidence     []DeliveryEvidence `json:"delivery_evidence,omitempty"`
}

// ConceptComparison 是某概念的正式比较结果。只有已批准映射的国家
// 才进入比较；其余国家显式列入“尚无共识”。
type ConceptComparison struct {
	Concept              string                   `json:"concept"`
	Comparable           []CountryConceptEvidence `json:"comparable"`
	NoConsensusCountries []string                 `json:"no_consensus_countries"`
	// DeliveryVote 只汇总真正有原始出处的开设方式：
	// standalone / embedded / mixed / unknown，且按国家计票，避免虚假多数。
	DeliveryVote map[string]int `json:"delivery_vote,omitempty"`
}

// CompareConcept 基于全部已批准映射生成跨体系正式比较。
func (s *Service) CompareConcept(concept string) (ConceptComparison, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.buildComparisonLocked(concept, nil)
	if len(out.Comparable) == 0 {
		return out, fmt.Errorf("%w: 概念 %s 尚无已批准映射，无法进行正式比较", ErrState, concept)
	}
	return out, nil
}

// ---------- 口径版本 ----------

// CaliberInput 创建口径版本。
type CaliberInput struct {
	ID                 string             `json:"id"`
	Label              string             `json:"label"`
	Notes              string             `json:"notes,omitempty"`
	ApprovedMappingIDs []string           `json:"approved_mapping_ids"`
	Metrics            []MetricDefinition `json:"metrics"`
}

// CreateCaliber 冻结一组已批准映射与指标定义。
func (s *Service) CreateCaliber(in CaliberInput) (CaliberVersion, error) {
	if in.ID == "" || in.Label == "" {
		return CaliberVersion{}, fmt.Errorf("%w: 口径 ID 与标签必填", ErrInvalid)
	}
	if len(in.ApprovedMappingIDs) == 0 {
		return CaliberVersion{}, fmt.Errorf("%w: 口径至少包含一条已批准映射", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, id := range in.ApprovedMappingIDs {
		if seen[id] {
			return CaliberVersion{}, fmt.Errorf("%w: 映射 %s 在口径中重复", ErrInvalid, id)
		}
		seen[id] = true
	}
	metricSeen := map[string]bool{}
	for _, md := range in.Metrics {
		if md.Code == "" || md.Name == "" || md.Definition == "" {
			return CaliberVersion{}, fmt.Errorf("%w: 指标代码、名称与统计定义必填", ErrInvalid)
		}
		if metricSeen[md.Code] {
			return CaliberVersion{}, fmt.Errorf("%w: 指标 %s 重复", ErrInvalid, md.Code)
		}
		metricSeen[md.Code] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.calibers[in.ID]; ok {
		return CaliberVersion{}, fmt.Errorf("%w: 口径 %s 已存在，定义变更须建立新口径", ErrConflict, in.ID)
	}
	for _, id := range in.ApprovedMappingIDs {
		m, ok := s.mappings[id]
		if !ok {
			return CaliberVersion{}, fmt.Errorf("%w: 映射 %s 不存在", ErrInvalid, id)
		}
		if m.Status != MappingApproved {
			return CaliberVersion{}, fmt.Errorf("%w: 映射 %s 状态为 %s，只有已批准映射可进入口径", ErrInvalid, id, m.Status)
		}
	}
	cv := CaliberVersion{
		ID:                 in.ID,
		Label:              in.Label,
		Notes:              in.Notes,
		ApprovedMappingIDs: append([]string(nil), in.ApprovedMappingIDs...),
		Metrics:            append([]MetricDefinition(nil), in.Metrics...),
		CreatedOn:          s.stamp(),
	}
	sort.Strings(cv.ApprovedMappingIDs)
	s.calibers[cv.ID] = cv
	return cv, nil
}

// MetricFingerprint 返回口径内某指标统计定义的指纹；
// 试点数据采集时记录该指纹，阶段入库时据此发现定义漂移。
func (cv CaliberVersion) MetricFingerprint(code string) string {
	for _, m := range cv.Metrics {
		if m.Code == code {
			sum := sha256.Sum256([]byte(m.Code + "|" + m.Definition + "|" + m.Unit))
			return hex.EncodeToString(sum[:])[:12]
		}
	}
	return ""
}

// ---------- 改革试点 ----------

// PilotInput 创建试点。
type PilotInput struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	CaliberID      string          `json:"caliber_id"`
	Plan           string          `json:"plan"`
	InstitutionIDs []string        `json:"institution_ids"`
	Cohorts        []TeacherCohort `json:"cohorts"`
	Indicators     []string        `json:"indicators"`
}

// CreatePilot 创建与口径版本绑定的试点。
func (s *Service) CreatePilot(in PilotInput) (Pilot, error) {
	if in.ID == "" || in.Name == "" || in.CaliberID == "" || in.Plan == "" {
		return Pilot{}, fmt.Errorf("%w: 试点 ID、名称、口径与方案必填", ErrInvalid)
	}
	if len(in.InstitutionIDs) == 0 || len(in.Cohorts) == 0 || len(in.Indicators) == 0 {
		return Pilot{}, fmt.Errorf("%w: 参与院校、教师群体与观察指标均不能为空", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cv, ok := s.calibers[in.CaliberID]
	if !ok {
		return Pilot{}, fmt.Errorf("%w: 口径 %s 不存在", ErrInvalid, in.CaliberID)
	}
	if _, exists := s.pilots[in.ID]; exists {
		return Pilot{}, fmt.Errorf("%w: 试点 %s 已存在", ErrConflict, in.ID)
	}
	for _, id := range in.InstitutionIDs {
		if _, ok := s.institutions[id]; !ok {
			return Pilot{}, fmt.Errorf("%w: 参与院校 %s 未登记", ErrInvalid, id)
		}
	}
	defined := map[string]bool{}
	for _, m := range cv.Metrics {
		defined[m.Code] = true
	}
	for _, code := range in.Indicators {
		if !defined[code] {
			return Pilot{}, fmt.Errorf("%w: 指标 %s 不在口径 %s 的定义中", ErrInvalid, code, in.CaliberID)
		}
	}
	for _, c := range in.Cohorts {
		if c.Label == "" || c.Size <= 0 {
			return Pilot{}, fmt.Errorf("%w: 教师群体标签与人数必填", ErrInvalid)
		}
	}
	p := Pilot{
		ID:             in.ID,
		Name:           in.Name,
		CaliberID:      in.CaliberID,
		Plan:           in.Plan,
		InstitutionIDs: append([]string(nil), in.InstitutionIDs...),
		Cohorts:        append([]TeacherCohort(nil), in.Cohorts...),
		Indicators:     append([]string(nil), in.Indicators...),
		CreatedOn:      s.stamp(),
	}
	sort.Strings(p.InstitutionIDs)
	s.pilots[p.ID] = p
	return p, nil
}

// StageInput 录入试点阶段结果。
type StageInput struct {
	ID         string             `json:"id"`
	Label      string             `json:"label"`
	Results    []IndicatorResult  `json:"results"`
	Conclusion *OutcomeConclusion `json:"conclusion,omitempty"`
}

// AddStage 录入阶段结果并强制执行阻断规则：
// 样本缺失或统计定义漂移时，该阶段不得附带成效结论；系统也从不自动生成结论。
func (s *Service) AddStage(pilotID string, in StageInput) (PilotStage, error) {
	if in.ID == "" || in.Label == "" {
		return PilotStage{}, fmt.Errorf("%w: 阶段 ID 与标签必填", ErrInvalid)
	}
	if len(in.Results) == 0 {
		return PilotStage{}, fmt.Errorf("%w: 阶段至少要有一条指标结果", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pilots[pilotID]
	if !ok {
		return PilotStage{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, pilotID)
	}
	if _, dup := s.stages[in.ID]; dup {
		return PilotStage{}, fmt.Errorf("%w: 阶段 %s 已存在", ErrConflict, in.ID)
	}
	cv := s.calibers[p.CaliberID]
	want := map[string]bool{}
	for _, code := range p.Indicators {
		want[code] = true
	}
	seen := map[string]bool{}
	var blockers []string
	for _, r := range in.Results {
		if !want[r.MetricCode] {
			return PilotStage{}, fmt.Errorf("%w: 指标 %s 不属于试点 %s 绑定的口径", ErrInvalid, r.MetricCode, pilotID)
		}
		if seen[r.MetricCode] {
			return PilotStage{}, fmt.Errorf("%w: 指标 %s 在阶段中重复", ErrInvalid, r.MetricCode)
		}
		seen[r.MetricCode] = true
		fp := cv.MetricFingerprint(r.MetricCode)
		switch {
		case r.DefinitionFingerprint == "":
			blockers = append(blockers, fmt.Sprintf("指标 %s 的结果缺少统计定义指纹，无法确认口径一致", r.MetricCode))
		case r.DefinitionFingerprint != fp:
			blockers = append(blockers, fmt.Sprintf("指标 %s 的统计定义已改变（记录指纹 %s ≠ 口径 %s）", r.MetricCode, r.DefinitionFingerprint, fp))
		}
		if r.ExpectedSample > 0 && r.SampleSize < r.ExpectedSample {
			reason := r.MissingReason
			if reason == "" {
				reason = "未说明"
			}
			blockers = append(blockers, fmt.Sprintf("指标 %s 样本缺失：%d/%d（%s）", r.MetricCode, r.SampleSize, r.ExpectedSample, reason))
		}
		if r.Value == nil && r.MissingReason == "" {
			blockers = append(blockers, fmt.Sprintf("指标 %s 无取值且未登记缺失原因", r.MetricCode))
		}
	}
	for code := range want {
		if !seen[code] {
			blockers = append(blockers, fmt.Sprintf("指标 %s 本阶段未上报结果", code))
		}
	}
	sort.Strings(blockers)

	if in.Conclusion != nil {
		if len(blockers) > 0 {
			return PilotStage{}, fmt.Errorf("%w: 存在 %d 项阻断，样本/定义问题未解决前不得记录成效结论", ErrState, len(blockers))
		}
		c := *in.Conclusion
		if c.RecordedBy == "" || c.Decision == "" || c.Rationale == "" {
			return PilotStage{}, fmt.Errorf("%w: 结论必须由专家显式给出决策（continue/modify/stop）与理由", ErrInvalid)
		}
		switch c.Decision {
		case "continue", "modify", "stop":
		default:
			return PilotStage{}, fmt.Errorf("%w: 决策只能是 continue/modify/stop", ErrInvalid)
		}
		if c.On == "" {
			c.On = s.stamp()
		}
		in.Conclusion = &c
	}

	st := PilotStage{
		ID:         in.ID,
		Label:      in.Label,
		RecordedOn: s.stamp(),
		Results:    append([]IndicatorResult(nil), in.Results...),
		Blockers:   blockers,
		Conclusion: in.Conclusion,
	}
	sort.Slice(st.Results, func(i, j int) bool { return st.Results[i].MetricCode < st.Results[j].MetricCode })
	s.stages[st.ID] = st
	s.pilotStages[pilotID] = append(s.pilotStages[pilotID], st.ID)
	return st, nil
}

// ---------- 政策建议、提案与证据链 ----------

// RecommendationInput 提交政策建议，必须精确引用口径、试点阶段与映射版本。
type RecommendationInput struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Detail     string   `json:"detail"`
	PilotID    string   `json:"pilot_id"`
	StageIDs   []string `json:"stage_ids"`
	MappingIDs []string `json:"mapping_ids"`
}

// AddRecommendation 登记政策建议。被引用的阶段必须已有显式结论，
// 且映射必须是该口径当时包含的已批准版本（日后被修订替代也不影响本记录）。
func (s *Service) AddRecommendation(in RecommendationInput) (PolicyRecommendation, error) {
	if in.ID == "" || in.Title == "" || in.Detail == "" || in.PilotID == "" {
		return PolicyRecommendation{}, fmt.Errorf("%w: 建议 ID、标题、内容与试点必填", ErrInvalid)
	}
	if len(in.StageIDs) == 0 || len(in.MappingIDs) == 0 {
		return PolicyRecommendation{}, fmt.Errorf("%w: 建议必须引用阶段结果与映射版本", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pilots[in.PilotID]
	if !ok {
		return PolicyRecommendation{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, in.PilotID)
	}
	if _, dup := s.recs[in.ID]; dup {
		return PolicyRecommendation{}, fmt.Errorf("%w: 建议 %s 已存在", ErrConflict, in.ID)
	}
	cv := s.calibers[p.CaliberID]
	inCaliber := map[string]bool{}
	for _, id := range cv.ApprovedMappingIDs {
		inCaliber[id] = true
	}
	for _, id := range in.StageIDs {
		st, ok := s.stages[id]
		if !ok {
			return PolicyRecommendation{}, fmt.Errorf("%w: 阶段 %s 不存在", ErrInvalid, id)
		}
		if !contains(s.pilotStages[in.PilotID], st.ID) {
			return PolicyRecommendation{}, fmt.Errorf("%w: 阶段 %s 不属于试点 %s", ErrInvalid, id, in.PilotID)
		}
		if st.Conclusion == nil {
			return PolicyRecommendation{}, fmt.Errorf("%w: 阶段 %s 尚无显式成效结论，不能支撑政策建议", ErrState, id)
		}
	}
	for _, id := range in.MappingIDs {
		m, ok := s.mappings[id]
		if !ok {
			return PolicyRecommendation{}, fmt.Errorf("%w: 映射 %s 不存在", ErrInvalid, id)
		}
		if m.Status != MappingApproved {
			return PolicyRecommendation{}, fmt.Errorf("%w: 映射 %s 当前状态 %s，不能作为正式依据", ErrInvalid, id, m.Status)
		}
		if !inCaliber[id] {
			return PolicyRecommendation{}, fmt.Errorf("%w: 映射 %s 不属于口径 %s", ErrInvalid, id, cv.ID)
		}
	}
	rec := PolicyRecommendation{
		ID:         in.ID,
		Title:      in.Title,
		Detail:     in.Detail,
		CaliberID:  cv.ID,
		PilotID:    in.PilotID,
		StageIDs:   append([]string(nil), in.StageIDs...),
		MappingIDs: append([]string(nil), in.MappingIDs...),
		CreatedOn:  s.stamp(),
	}
	sort.Strings(rec.StageIDs)
	sort.Strings(rec.MappingIDs)
	s.recs[rec.ID] = rec
	return rec, nil
}

// ProposalInput 决策者提出的一次课程调整。
type ProposalInput struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	ProposedBy        string   `json:"proposed_by"`
	RecommendationIDs []string `json:"recommendation_ids"`
}

// AddProposal 登记课程调整提案并关联政策建议。
func (s *Service) AddProposal(in ProposalInput) (PolicyProposal, error) {
	if in.ID == "" || in.Title == "" || in.Description == "" || in.ProposedBy == "" {
		return PolicyProposal{}, fmt.Errorf("%w: 提案 ID、标题、内容与提出人必填", ErrInvalid)
	}
	if len(in.RecommendationIDs) == 0 {
		return PolicyProposal{}, fmt.Errorf("%w: 提案至少关联一条政策建议", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.proposals[in.ID]; dup {
		return PolicyProposal{}, fmt.Errorf("%w: 提案 %s 已存在", ErrConflict, in.ID)
	}
	var caliberID string
	for _, id := range in.RecommendationIDs {
		rec, ok := s.recs[id]
		if !ok {
			return PolicyProposal{}, fmt.Errorf("%w: 建议 %s 不存在", ErrInvalid, id)
		}
		// 一次课程调整的证据链必须锚定同一口径；跨口径证据应先在新口径下重审，
		// 再形成新的建议，避免比较范围与指标定义在同一条链里含混。
		if caliberID == "" {
			caliberID = rec.CaliberID
		} else if rec.CaliberID != caliberID {
			return PolicyProposal{}, fmt.Errorf("%w: 提案不能混用口径 %s 与 %s", ErrInvalid, caliberID, rec.CaliberID)
		}
	}
	pp := PolicyProposal{
		ID:                in.ID,
		Title:             in.Title,
		Description:       in.Description,
		ProposedBy:        in.ProposedBy,
		On:                s.stamp(),
		RecommendationIDs: append([]string(nil), in.RecommendationIDs...),
	}
	sort.Strings(pp.RecommendationIDs)
	s.proposals[pp.ID] = pp
	return pp, nil
}

// StageEvidence 阶段在证据链中的呈现，含阻断项与结论。
type StageEvidence struct {
	PilotStage
	PilotID string `json:"pilot_id"`
}

// RecommendationEvidence 政策建议展开后的证据细节。
type RecommendationEvidence struct {
	PolicyRecommendation
	Mappings []Mapping       `json:"mappings"`
	Stages   []StageEvidence `json:"stages"`
}

// EvidenceChain 是一次课程调整提案的完整证据链。
type EvidenceChain struct {
	Proposal        PolicyProposal           `json:"proposal"`
	Caliber         CaliberVersion           `json:"caliber"`
	Recommendations []RecommendationEvidence `json:"recommendations"`
	Comparisons     []ConceptComparison      `json:"comparisons"`
	// BlockedStages 列出仍存在阻断项的阶段；这些阶段不能支持继续/改动/停止的判断。
	BlockedStages []StageEvidence `json:"blocked_stages"`
	// Decisions 汇总证据链中已显式记录的阶段决策。
	Decisions []string `json:"decisions"`
}

// EvidenceChainFor 沿提案回溯证据链：建议 → 口径/映射版本（含已被替代的旧版）→
// 试点阶段结论 → 各概念的正式可比性与无共识国家。
func (s *Service) EvidenceChainFor(proposalID string) (EvidenceChain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pp, ok := s.proposals[proposalID]
	if !ok {
		return EvidenceChain{}, fmt.Errorf("%w: 提案 %s", ErrNotFound, proposalID)
	}
	chain := EvidenceChain{Proposal: pp}
	concepts := map[string]bool{}
	caliberID := ""
	seenStage := map[string]bool{}
	pilots := map[string]bool{}

	for _, rid := range pp.RecommendationIDs {
		rec := s.recs[rid]
		rev := RecommendationEvidence{PolicyRecommendation: rec}
		if caliberID == "" {
			caliberID = rec.CaliberID
		}
		pilots[rec.PilotID] = true
		for _, mid := range rec.MappingIDs {
			rev.Mappings = append(rev.Mappings, s.mappings[mid])
			concepts[s.mappings[mid].Concept] = true
		}
		for _, sid := range rec.StageIDs {
			st := s.stages[sid]
			se := StageEvidence{PilotStage: st, PilotID: rec.PilotID}
			rev.Stages = append(rev.Stages, se)
			seenStage[sid] = true
			if len(st.Blockers) > 0 {
				chain.BlockedStages = append(chain.BlockedStages, se)
			}
			if st.Conclusion != nil {
				chain.Decisions = append(chain.Decisions,
					fmt.Sprintf("%s: %s", st.ID, st.Conclusion.Decision))
			}
		}
		sort.Slice(rev.Mappings, func(i, j int) bool { return rev.Mappings[i].ID < rev.Mappings[j].ID })
		chain.Recommendations = append(chain.Recommendations, rev)
	}
	// 关联试点下即使未被某条建议直接引用的阻断阶段，也要让决策者看见：
	// 样本缺失或定义漂移的部分不能被静默隐藏。
	pilotIDs := make([]string, 0, len(pilots))
	for pid := range pilots {
		pilotIDs = append(pilotIDs, pid)
	}
	sort.Strings(pilotIDs)
	for _, pid := range pilotIDs {
		for _, sid := range s.pilotStages[pid] {
			st := s.stages[sid]
			if len(st.Blockers) == 0 || seenStage[sid] {
				continue
			}
			chain.BlockedStages = append(chain.BlockedStages,
				StageEvidence{PilotStage: st, PilotID: pid})
		}
	}
	sort.Slice(chain.BlockedStages, func(i, j int) bool {
		if chain.BlockedStages[i].PilotID != chain.BlockedStages[j].PilotID {
			return chain.BlockedStages[i].PilotID < chain.BlockedStages[j].PilotID
		}
		return chain.BlockedStages[i].ID < chain.BlockedStages[j].ID
	})
	if caliberID != "" {
		chain.Caliber = s.calibers[caliberID]
	}
	cs := make([]string, 0, len(concepts))
	for c := range concepts {
		cs = append(cs, c)
	}
	sort.Strings(cs)
	for _, c := range cs {
		// 证据链锚定在冻结的口径上：只展示口径内映射版本构成的比较，
		// 新批准但不在该口径的映射不会改写历史证据链。
		cmp := s.buildComparisonLocked(c, s.calibers[caliberID].ApprovedMappingIDs)
		chain.Comparisons = append(chain.Comparisons, cmp)
	}
	sort.Slice(chain.Recommendations, func(i, j int) bool {
		return chain.Recommendations[i].ID < chain.Recommendations[j].ID
	})
	sort.Strings(chain.Decisions)
	return chain, nil
}

// buildComparisonLocked 假定已持有读锁，构造概念比较。
// allow 为 nil 时纳入全部已批准映射（用于实时正式比较）；
// 传入口径冻结的映射 ID 集时，只纳入这些版本（即使其中有的日后已被替代），
// 以保证历史证据链不被后续修订改写。
func (s *Service) buildComparisonLocked(concept string, allow []string) ConceptComparison {
	out := ConceptComparison{Concept: concept, DeliveryVote: map[string]int{}}
	covered := map[string]bool{}
	restrict := allow != nil
	allowed := map[string]bool{}
	for _, id := range allow {
		allowed[id] = true
	}
	ids := make([]string, 0, len(s.mappings))
	for id := range s.mappings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m := s.mappings[id]
		if m.Concept != concept {
			continue
		}
		if !restrict {
			if m.Status != MappingApproved {
				continue
			}
		} else if !allowed[id] {
			continue
		}
		ev := CountryConceptEvidence{
			CountryCode: m.Anchor.CountryCode, MappingID: m.ID, MappingStatus: m.Status,
			Relationship: m.Relationship, Confidence: m.Confidence, Scope: m.Scope,
		}
		for _, o := range m.Objections {
			if o.Resolution == "" {
				ev.UnresolvedObjections = append(ev.UnresolvedObjections, o)
			}
		}
		if m.Anchor.Kind == "course_topic" {
			modes := map[string]bool{}
			for _, c := range s.courses {
				inst := s.institutions[c.InstitutionID]
				if inst.CountryCode != m.Anchor.CountryCode || !contains(c.TopicTags, m.Anchor.Ref) {
					continue
				}
				if mode, ok := c.DeliveryByTopic[m.Anchor.Ref]; ok {
					modes[string(mode)] = true
					src := SourceReference{}
					if len(c.Sources) > 0 {
						src = c.Sources[0]
					}
					ev.DeliveryEvidence = append(ev.DeliveryEvidence, DeliveryEvidence{
						CourseID: c.ID, InstitutionID: c.InstitutionID,
						ProgramVersion: c.ProgramVersion, Mode: mode, Source: src,
					})
				}
			}
			for mode := range modes {
				ev.DeliveryModes = append(ev.DeliveryModes, mode)
			}
			sort.Strings(ev.DeliveryModes)
			sort.Slice(ev.DeliveryEvidence, func(i, j int) bool {
				return ev.DeliveryEvidence[i].CourseID < ev.DeliveryEvidence[j].CourseID
			})
			switch {
			case len(modes) > 1:
				out.DeliveryVote["mixed"]++
			case modes[string(DeliveryStandalone)]:
				out.DeliveryVote[string(DeliveryStandalone)]++
			case modes[string(DeliveryEmbedded)]:
				out.DeliveryVote[string(DeliveryEmbedded)]++
			default:
				out.DeliveryVote["unknown"]++
			}
		}
		out.Comparable = append(out.Comparable, ev)
		covered[m.Anchor.CountryCode] = true
	}
	codes := make([]string, 0, len(s.countries))
	for code := range s.countries {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if !covered[code] {
			out.NoConsensusCountries = append(out.NoConsensusCountries, code)
		}
	}
	return out
}

// ---------- 只读查询 ----------

// GetCourse 返回原始课程记录。
func (s *Service) GetCourse(id string) (Course, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.courses[id]
	if !ok {
		return Course{}, fmt.Errorf("%w: 课程 %s", ErrNotFound, id)
	}
	return c, nil
}

// GetMapping 返回映射（含修订链上的任意版本）。
func (s *Service) GetMapping(id string) (Mapping, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.mappings[id]
	if !ok {
		return Mapping{}, fmt.Errorf("%w: 映射 %s", ErrNotFound, id)
	}
	return m, nil
}

// GetPilot 返回试点。
func (s *Service) GetPilot(id string) (Pilot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pilots[id]
	if !ok {
		return Pilot{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, id)
	}
	stages := make([]PilotStage, 0, len(s.pilotStages[id]))
	for _, sid := range s.pilotStages[id] {
		stages = append(stages, s.stages[sid])
	}
	p.Stages = stages
	return p, nil
}

// GetCaliber 返回口径版本。
func (s *Service) GetCaliber(id string) (CaliberVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cv, ok := s.calibers[id]
	if !ok {
		return CaliberVersion{}, fmt.Errorf("%w: 口径 %s", ErrNotFound, id)
	}
	return cv, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func findItem(fw CompetencyFramework, code string) bool {
	for _, it := range fw.Items {
		if it.Code == code {
			return true
		}
	}
	return false
}
