package evidence

import (
	"fmt"
	"strings"
	"time"
)

// Service 在 Store 之上执行业务规则。
type Service struct {
	store *Store
	now   func() time.Time
}

// NewService 创建服务。
func NewService(st *Store) *Service {
	return &Service{store: st, now: time.Now}
}

func nonempty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s 不能为空", ErrInvalid, field)
	}
	return nil
}

// ---- 制度背景与院校 ----

func (s *Service) AddSystem(sys SystemContext) error {
	if err := nonempty("id", sys.ID); err != nil {
		return err
	}
	if err := nonempty("country", sys.Country); err != nil {
		return err
	}
	unlock := s.store.lock()
	defer unlock()
	if _, ok := s.store.Systems[sys.ID]; ok {
		return fmt.Errorf("%w: 制度背景 %s", ErrAlreadyExists, sys.ID)
	}
	s.store.Systems[sys.ID] = sys
	return s.store.persist()
}

func (s *Service) AddInstitution(inst Institution) error {
	if err := nonempty("id", inst.ID); err != nil {
		return err
	}
	if err := nonempty("name", inst.Name); err != nil {
		return err
	}
	unlock := s.store.lock()
	defer unlock()
	if _, ok := s.store.Institutions[inst.ID]; ok {
		return fmt.Errorf("%w: 院校 %s", ErrAlreadyExists, inst.ID)
	}
	if _, ok := s.store.Systems[inst.SystemID]; !ok {
		return fmt.Errorf("%w: 制度背景 %s 不存在", ErrInvalid, inst.SystemID)
	}
	if inst.Country == "" {
		inst.Country = s.store.Systems[inst.SystemID].Country
	}
	s.store.Institutions[inst.ID] = inst
	return s.store.persist()
}

// ---- 出处 ----

func (s *Service) AddSource(src Source) error {
	if err := nonempty("id", src.ID); err != nil {
		return err
	}
	if err := nonempty("citation", src.Citation); err != nil {
		return err
	}
	unlock := s.store.lock()
	defer unlock()
	if _, ok := s.store.Sources[src.ID]; ok {
		return fmt.Errorf("%w: 出处 %s", ErrAlreadyExists, src.ID)
	}
	if src.RetrievedAt.IsZero() {
		src.RetrievedAt = s.now()
	}
	s.store.Sources[src.ID] = src
	return s.store.persist()
}

// ---- 能力框架 ----

func (s *Service) AddFramework(fw Framework) error {
	if err := nonempty("id", fw.ID); err != nil {
		return err
	}
	if err := nonempty("version", fw.Version); err != nil {
		return err
	}
	unlock := s.store.lock()
	defer unlock()
	if _, ok := s.store.Frameworks[fw.ID]; ok {
		return fmt.Errorf("%w: 框架 %s", ErrAlreadyExists, fw.ID)
	}
	if _, ok := s.store.Systems[fw.SystemID]; !ok {
		return fmt.Errorf("%w: 制度背景 %s 不存在", ErrInvalid, fw.SystemID)
	}
	if len(fw.Domains) == 0 {
		return fmt.Errorf("%w: 框架至少包含一个素养领域", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, d := range fw.Domains {
		if d.Key == "" || d.Name == "" {
			return fmt.Errorf("%w: 素养领域的 key 与名称不能为空", ErrInvalid)
		}
		if seen[d.Key] {
			return fmt.Errorf("%w: 素养领域 %s 重复", ErrInvalid, d.Key)
		}
		seen[d.Key] = true
	}
	s.store.Frameworks[fw.ID] = fw
	return s.store.persist()
}

// ---- 课程版本 ----

// AddCourseInput 是登记课程版本的输入。
type AddCourseInput struct {
	ID            string
	Version       string
	InstitutionID string
	Title         string
	LocalTitle    string
	Credits       float64
	CreditUnit    string
	Prerequisites []string
	AI            AIDelivery
	Practice      PracticeComponent
	FrameworkID   string
	SourceIDs     []string
	EffectiveFrom string
}

func validAIMode(mode string) bool {
	return mode == AIModeStandalone || mode == AIModeIntegrated || mode == AIModeNone
}

func (s *Service) AddCourse(in AddCourseInput) (CourseVersion, error) {
	if err := nonempty("id", in.ID); err != nil {
		return CourseVersion{}, err
	}
	if err := nonempty("version", in.Version); err != nil {
		return CourseVersion{}, err
	}
	if err := nonempty("title", in.Title); err != nil {
		return CourseVersion{}, err
	}
	if in.Credits < 0 {
		return CourseVersion{}, fmt.Errorf("%w: 学分不能为负", ErrInvalid)
	}
	if !validAIMode(in.AI.Mode) {
		return CourseVersion{}, fmt.Errorf("%w: AI 开设方式必须是 %s/%s/%s", ErrInvalid, AIModeStandalone, AIModeIntegrated, AIModeNone)
	}
	if in.AI.Mode == AIModeIntegrated && len(in.AI.IntegratedInto) == 0 {
		return CourseVersion{}, fmt.Errorf("%w: 融入式 AI 内容必须注明融入的课程", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()

	inst, ok := s.store.Institutions[in.InstitutionID]
	if !ok {
		return CourseVersion{}, fmt.Errorf("%w: 院校 %s 不存在", ErrInvalid, in.InstitutionID)
	}
	fw, ok := s.store.Frameworks[in.FrameworkID]
	if !ok {
		return CourseVersion{}, fmt.Errorf("%w: %s", ErrFrameworkMissing, in.FrameworkID)
	}
	if fw.SystemID != inst.SystemID {
		return CourseVersion{}, fmt.Errorf("%w: 能力框架 %s 不属于该院校所在制度", ErrInvalid, in.FrameworkID)
	}
	key := in.ID + "@" + in.Version
	if _, ok := s.store.Courses[key]; ok {
		return CourseVersion{}, fmt.Errorf("%w: 课程版本 %s", ErrAlreadyExists, key)
	}
	for _, pre := range in.Prerequisites {
		preCourse, ok := s.store.Courses[pre]
		if !ok {
			return CourseVersion{}, fmt.Errorf("%w: %s", ErrMissingPrerequisite, pre)
		}
		if preCourse.InstitutionID != in.InstitutionID {
			return CourseVersion{}, fmt.Errorf("%w: 先修课程 %s 不属于同一院校", ErrInvalid, pre)
		}
	}
	for _, sid := range in.SourceIDs {
		if _, ok := s.store.Sources[sid]; !ok {
			return CourseVersion{}, fmt.Errorf("%w: 出处 %s 不存在", ErrInvalid, sid)
		}
	}
	unit := in.CreditUnit
	if unit == "" {
		unit = "ECTS"
	}
	cv := CourseVersion{
		ID:            in.ID,
		Version:       in.Version,
		InstitutionID: in.InstitutionID,
		SystemID:      inst.SystemID,
		Title:         in.Title,
		LocalTitle:    in.LocalTitle,
		Credits:       in.Credits,
		CreditUnit:    unit,
		Prerequisites: append([]string(nil), in.Prerequisites...),
		AI:            in.AI,
		Practice:      in.Practice,
		FrameworkID:   in.FrameworkID,
		SourceIDs:     append([]string(nil), in.SourceIDs...),
		EffectiveFrom: in.EffectiveFrom,
		RegisteredAt:  s.now(),
	}
	s.store.Courses[key] = cv
	return cv, s.store.persist()
}

// ---- 持续专业发展 ----

func validCPDStage(stage string) bool {
	return stage == CPDStagePreservice || stage == CPDInduction || stage == CPDInService
}

func (s *Service) AddCPD(p CPDProgram) error {
	if err := nonempty("id", p.ID); err != nil {
		return err
	}
	if err := nonempty("name", p.Name); err != nil {
		return err
	}
	if !validCPDStage(p.Stage) {
		return fmt.Errorf("%w: CPD 阶段必须是 %s/%s/%s", ErrInvalid, CPDStagePreservice, CPDInduction, CPDInService)
	}
	unlock := s.store.lock()
	defer unlock()
	if _, ok := s.store.CPDs[p.ID]; ok {
		return fmt.Errorf("%w: CPD 项目 %s", ErrAlreadyExists, p.ID)
	}
	if _, ok := s.store.Systems[p.SystemID]; !ok {
		return fmt.Errorf("%w: 制度背景 %s 不存在", ErrInvalid, p.SystemID)
	}
	for _, sid := range p.SourceIDs {
		if _, ok := s.store.Sources[sid]; !ok {
			return fmt.Errorf("%w: 出处 %s 不存在", ErrInvalid, sid)
		}
	}
	s.store.CPDs[p.ID] = p
	return s.store.persist()
}

// ---- 只读查询 ----

func (s *Service) GetCourse(ref string) (CourseVersion, error) {
	unlock := s.store.rlock()
	defer unlock()
	cv, ok := s.store.Courses[ref]
	if !ok {
		return CourseVersion{}, fmt.Errorf("%w: 课程版本 %s", ErrNotFound, ref)
	}
	return cv, nil
}

func (s *Service) ListCourses() []CourseVersion {
	unlock := s.store.rlock()
	defer unlock()
	out := make([]CourseVersion, 0, len(s.store.Courses))
	for _, cv := range s.store.Courses {
		out = append(out, cv)
	}
	return out
}
