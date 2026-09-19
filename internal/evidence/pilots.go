package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// parseVersionedID 解析 "逻辑ID@版本号" 形式的实例 ID。
func parseVersionedID(id string) (string, int, bool) {
	at := strings.LastIndex(id, "@")
	if at <= 0 || at == len(id)-1 {
		return "", 0, false
	}
	v, err := strconv.Atoi(id[at+1:])
	if err != nil {
		return "", 0, false
	}
	return id[:at], v, true
}

// RegisterCalibrationInput 登记一个统计口径版本。
type RegisterCalibrationInput struct {
	LogicalID            string // 同一口径沿用，重新登记产生新版本
	CurriculumRefs       map[string]string
	FrameworkRefs        map[string]string
	Metrics              []MetricDefinition
	ExpectedInstitutions []string
	ExpectedCohorts      []string
	MinResponseRate      float64
}

// RegisterCalibration 冻结一次统计口径。对同一 LogicalID 重新登记时旧版标记被替代。
// 课程方案/院校/群体/指标都通过返回的口径版本绑定。
func (s *Service) RegisterCalibration(in RegisterCalibrationInput) (Calibration, error) {
	if in.LogicalID == "" {
		return Calibration{}, fmt.Errorf("%w: 口径逻辑 ID 不能为空", ErrInvalid)
	}
	if len(in.Metrics) == 0 {
		return Calibration{}, fmt.Errorf("%w: 口径至少包含一项观察指标定义", ErrInvalid)
	}
	for _, m := range in.Metrics {
		if m.Key == "" || m.Name == "" || m.Definition == "" || m.Unit == "" {
			return Calibration{}, fmt.Errorf("%w: 指标的 key/名称/定义/单位均不能为空", ErrInvalid)
		}
	}
	if len(in.ExpectedInstitutions) == 0 || len(in.ExpectedCohorts) == 0 {
		return Calibration{}, fmt.Errorf("%w: 口径必须声明预期院校与教师群体范围", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()
	for inst, ref := range in.CurriculumRefs {
		cv, ok := s.store.Courses[ref]
		if !ok {
			return Calibration{}, fmt.Errorf("%w: 院校 %s 绑定的课程版本 %s 不存在", ErrInvalid, inst, ref)
		}
		if cv.InstitutionID != inst {
			return Calibration{}, fmt.Errorf("%w: 课程版本 %s 不属于院校 %s", ErrInvalid, ref, inst)
		}
	}
	version := 0
	var previous *Calibration
	for _, c := range s.store.Calibrations {
		logical, v, ok := parseVersionedID(c.ID)
		if ok && logical == in.LogicalID && v > version {
			version = v
			previous = c
		}
	}
	version++
	cal := Calibration{
		ID:                   fmt.Sprintf("%s@%d", in.LogicalID, version),
		Version:              version,
		CurriculumRefs:       cloneStringMap(in.CurriculumRefs),
		FrameworkRefs:        cloneStringMap(in.FrameworkRefs),
		Metrics:              append([]MetricDefinition(nil), in.Metrics...),
		ExpectedInstitutions: append([]string(nil), in.ExpectedInstitutions...),
		ExpectedCohorts:      append([]string(nil), in.ExpectedCohorts...),
		MinResponseRate:      in.MinResponseRate,
		CreatedAt:            s.now(),
	}
	sort.Strings(cal.ExpectedInstitutions)
	sort.Strings(cal.ExpectedCohorts)
	if previous != nil {
		previous.SupersededBy = cal.ID
	}
	s.store.Calibrations[cal.ID] = &cal
	return cal, s.store.persist()
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// CalibrationFingerprint 是口径内容的稳定指纹，用于检测课程/框架/指标是否仍为同一口径。
func (c Calibration) Fingerprint() string {
	keys := make([]string, 0, len(c.CurriculumRefs))
	for k := range c.CurriculumRefs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	fmt.Fprintf(h, "v=%d|", c.Version)
	for _, k := range keys {
		fmt.Fprintf(h, "cur:%s=%s;", k, c.CurriculumRefs[k])
	}
	fkeys := make([]string, 0, len(c.FrameworkRefs))
	for k := range c.FrameworkRefs {
		fkeys = append(fkeys, k)
	}
	sort.Strings(fkeys)
	for _, k := range fkeys {
		fmt.Fprintf(h, "fw:%s=%s;", k, c.FrameworkRefs[k])
	}
	for _, m := range c.Metrics {
		fmt.Fprintf(h, "m:%s|%s|%s|%s;", m.Key, m.Name, m.Definition, m.Unit)
	}
	fmt.Fprintf(h, "|inst:%v|cohorts:%v|rr:%f", c.ExpectedInstitutions, c.ExpectedCohorts, c.MinResponseRate)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// CalibrationCurriculumRefs 返回口径绑定的课程版本引用（排序后）。
func (p Pilot) CalibrationCurriculumRefs(st *Store) []string {
	cal, ok := st.Calibrations[p.CalibrationID]
	if !ok {
		return nil
	}
	refs := make([]string, 0, len(cal.CurriculumRefs))
	for _, r := range cal.CurriculumRefs {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	return refs
}

// CreatePilotInput 创建试点的输入。
type CreatePilotInput struct {
	ID                        string
	Name                      string
	PlanName                  string
	CalibrationID             string
	ParticipatingInstitutions []string
	TeacherCohorts            []string
	Indicators                []string
}

// CreatePilot 建立试点，方案/院校/教师群体/指标与指定口径版本绑定。
func (s *Service) CreatePilot(in CreatePilotInput) (Pilot, error) {
	for field, v := range map[string]string{"id": in.ID, "name": in.Name, "plan_name": in.PlanName, "calibration_id": in.CalibrationID} {
		if v == "" {
			return Pilot{}, fmt.Errorf("%w: %s 不能为空", ErrInvalid, field)
		}
	}
	unlock := s.store.lock()
	defer unlock()
	if _, exists := s.store.Pilots[in.ID]; exists {
		return Pilot{}, fmt.Errorf("%w: 试点 %s", ErrAlreadyExists, in.ID)
	}
	cal, ok := s.store.Calibrations[in.CalibrationID]
	if !ok {
		return Pilot{}, fmt.Errorf("%w: 口径版本 %s 不存在", ErrCalibrationMissing, in.CalibrationID)
	}
	if cal.SupersededBy != "" {
		return Pilot{}, fmt.Errorf("%w: 口径 %s 已被 %s 替代，请用新口径建立试点", ErrCalibrationChanged, cal.ID, cal.SupersededBy)
	}
	metricSet := map[string]bool{}
	for _, m := range cal.Metrics {
		metricSet[m.Key] = true
	}
	if len(in.Indicators) == 0 {
		return Pilot{}, fmt.Errorf("%w: 试点至少选择一项观察指标", ErrInvalid)
	}
	for _, ind := range in.Indicators {
		if !metricSet[ind] {
			return Pilot{}, fmt.Errorf("%w: 指标 %s 不在口径 %s 中", ErrDefinitionChanged, ind, cal.ID)
		}
	}
	calInst := map[string]bool{}
	for _, inst := range cal.ExpectedInstitutions {
		calInst[inst] = true
	}
	for _, inst := range in.ParticipatingInstitutions {
		if !calInst[inst] {
			return Pilot{}, fmt.Errorf("%w: 院校 %s 不在口径样本范围", ErrPilotMismatch, inst)
		}
	}
	calCohort := map[string]bool{}
	for _, c := range cal.ExpectedCohorts {
		calCohort[c] = true
	}
	for _, c := range in.TeacherCohorts {
		if !calCohort[c] {
			return Pilot{}, fmt.Errorf("%w: 教师群体 %s 不在口径样本范围", ErrPilotMismatch, c)
		}
	}
	pilot := Pilot{
		ID:                        in.ID,
		Name:                      in.Name,
		PlanName:                  in.PlanName,
		CalibrationID:             cal.ID,
		CalibrationVersion:        cal.Version,
		ParticipatingInstitutions: append([]string(nil), in.ParticipatingInstitutions...),
		TeacherCohorts:            append([]string(nil), in.TeacherCohorts...),
		Indicators:                append([]string(nil), in.Indicators...),
		Stages:                    map[string]*PilotStageResult{},
		CreatedAt:                 s.now(),
	}
	sort.Strings(pilot.ParticipatingInstitutions)
	sort.Strings(pilot.TeacherCohorts)
	sort.Strings(pilot.Indicators)
	s.store.Pilots[in.ID] = &pilot
	return pilot, s.store.persist()
}

// ReportStageInput 上报试点某阶段观察数据。
type ReportStageInput struct {
	PilotID string
	Stage   string
	Period  string
	// PresentIndicators 本次实际采用定义上报的指标（必须等于试点指标全集）。
	PresentIndicators   []string
	Observations        map[string]float64
	MissingInstitutions []string
	MissingCohorts      []string
	ResponseRates       map[string]float64
}

// ReportStage 接收阶段数据，并按口径闸门做完整性校验；校验不过则拒绝写入。
// 该接口绝不生成成效结论——结论只能由 RecordConclusion 在闸门通过后显式登记。
func (s *Service) ReportStage(in ReportStageInput) (*PilotStageResult, error) {
	if in.Stage == "" {
		return nil, fmt.Errorf("%w: 阶段标识不能为空", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()
	pilot, ok := s.store.Pilots[in.PilotID]
	if !ok {
		return nil, fmt.Errorf("%w: 试点 %s", ErrNotFound, in.PilotID)
	}
	cal, ok := s.store.Calibrations[pilot.CalibrationID]
	if !ok {
		return nil, ErrCalibrationMissing
	}
	if cal.SupersededBy != "" {
		return nil, fmt.Errorf("%w: 口径 %s 已被 %s 替代，阶段数据须按新口径重新组织", ErrCalibrationChanged, cal.ID, cal.SupersededBy)
	}
	if _, exists := pilot.Stages[in.Stage]; exists {
		return nil, fmt.Errorf("%w: 阶段 %s 已上报；修订请登记新阶段或新口径", ErrAlreadyExists, in.Stage)
	}

	// 闸门 1：指标集必须与口径/试点完全一致（缺失或多出口径都拒绝）。
	defined := map[string]MetricDefinition{}
	for _, m := range cal.Metrics {
		defined[m.Key] = m
	}
	present := map[string]bool{}
	for _, k := range in.PresentIndicators {
		present[k] = true
	}
	for _, k := range pilot.Indicators {
		if !present[k] {
			return nil, fmt.Errorf("%w: 阶段 %s 缺少指标 %s 的定义或数据", ErrDefinitionChanged, in.Stage, k)
		}
	}
	for k := range in.Observations {
		if !present[k] {
			return nil, fmt.Errorf("%w: 观察值 %s 未声明采用的指标定义", ErrIndicatorNotInScope, k)
		}
		if _, ok := defined[k]; !ok {
			return nil, fmt.Errorf("%w: 指标 %s 不在口径 %s 中", ErrDefinitionChanged, k, cal.ID)
		}
	}

	// 闸门 2：样本完整性——预期院校与教师群体不得缺失。
	missingInst := map[string]bool{}
	for _, inst := range in.MissingInstitutions {
		missingInst[inst] = true
	}
	for _, inst := range cal.ExpectedInstitutions {
		if missingInst[inst] {
			return nil, fmt.Errorf("%w: 院校 %s 缺失", ErrSampleIncomplete, inst)
		}
	}
	missingCoh := map[string]bool{}
	for _, c := range in.MissingCohorts {
		missingCoh[c] = true
	}
	for _, c := range cal.ExpectedCohorts {
		if missingCoh[c] {
			return nil, fmt.Errorf("%w: 教师群体 %s 缺失", ErrSampleIncomplete, c)
		}
	}

	// 闸门 3：应答率不低于口径下限。
	if cal.MinResponseRate > 0 {
		for inst, rate := range in.ResponseRates {
			if rate < cal.MinResponseRate {
				return nil, fmt.Errorf("%w: 院校 %s 应答率 %.2f 低于下限 %.2f", ErrSampleIncomplete, inst, rate, cal.MinResponseRate)
			}
		}
	}

	res := &PilotStageResult{
		Stage:               in.Stage,
		Period:              in.Period,
		PresentIndicators:   append([]string(nil), in.PresentIndicators...),
		Observations:        cloneFloatMap(in.Observations),
		MissingInstitutions: append([]string(nil), in.MissingInstitutions...),
		MissingCohorts:      append([]string(nil), in.MissingCohorts...),
		ResponseRates:       cloneFloatMap(in.ResponseRates),
		ReportedAt:          s.now(),
	}
	sort.Strings(res.PresentIndicators)
	pilot.Stages[in.Stage] = res
	return res, s.store.persist()
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	if in == nil {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// RecordConclusionInput 登记阶段结论。
type RecordConclusionInput struct {
	PilotID  string
	Stage    string
	Decision string // continue | adjust | stop
	Author   string
	Comment  string
}

// RecordConclusion 在样本与口径闸门通过的前提下，为阶段登记"继续/改动/停止"结论。
func (s *Service) RecordConclusion(in RecordConclusionInput) (Pilot, error) {
	valid := map[string]bool{"continue": true, "adjust": true, "stop": true}
	if !valid[in.Decision] {
		return Pilot{}, fmt.Errorf("%w: 结论必须是 continue/adjust/stop", ErrInvalid)
	}
	if in.Author == "" {
		return Pilot{}, fmt.Errorf("%w: 结论作者不能为空", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()
	pilot, ok := s.store.Pilots[in.PilotID]
	if !ok {
		return Pilot{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, in.PilotID)
	}
	cal, ok := s.store.Calibrations[pilot.CalibrationID]
	if !ok {
		return Pilot{}, ErrCalibrationMissing
	}
	if cal.SupersededBy != "" {
		return Pilot{}, fmt.Errorf("%w: 口径 %s 已被 %s 替代，禁止沿用旧口径下结论", ErrCalibrationChanged, cal.ID, cal.SupersededBy)
	}
	res, ok := pilot.Stages[in.Stage]
	if !ok {
		return Pilot{}, fmt.Errorf("%w: 阶段 %s 尚无通过闸门的上报数据，不能下结论", ErrSampleIncomplete, in.Stage)
	}
	if res.Conclusion != "" {
		return Pilot{}, fmt.Errorf("%w: 阶段 %s 结论已存在", ErrAlreadyExists, in.Stage)
	}
	now := s.now()
	res.Conclusion = in.Decision
	res.ConclusionAuthor = in.Author
	res.ConcludedAt = &now
	res.ConclusionCalibrationVer = cal.Version
	if in.Comment != "" {
		res.Conclusion = in.Decision + "：" + in.Comment
	}
	return *pilot, s.store.persist()
}

// GetPilot 返回试点（含阶段结果）。
func (s *Service) GetPilot(id string) (Pilot, error) {
	unlock := s.store.rlock()
	defer unlock()
	p, ok := s.store.Pilots[id]
	if !ok {
		return Pilot{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, id)
	}
	return *p, nil
}

// CalibrationByID 供需要从其他聚合访问口径的场景使用。
func (s *Service) CalibrationByID(id string) (*Calibration, bool) {
	unlock := s.store.rlock()
	defer unlock()
	c, ok := s.store.Calibrations[id]
	return c, ok
}
