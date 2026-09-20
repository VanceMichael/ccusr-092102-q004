package curriculum_test

import (
	"errors"
	"strings"
	"testing"

	curriculum "example.com/teacher-curriculum-evidence"
)

// seedWorld 登记一个含四国的最小证据世界：
// CN 独立设课、DE 融入课程、FI 两种方式并存、BR 不提交任何映射。
func seedWorld(t *testing.T) *curriculum.Service {
	t.Helper()
	s := curriculum.NewService()

	must(t, s.RegisterCountry(curriculum.Country{Code: "CN", Name: "中国", SystemContext: "公费师范+教师资格证制度", EducationStage: "中学"}))
	must(t, s.RegisterCountry(curriculum.Country{Code: "DE", Name: "德国", SystemContext: "两阶段+国家考试", EducationStage: "文理中学"}))
	must(t, s.RegisterCountry(curriculum.Country{Code: "FI", Name: "芬兰", SystemContext: "硕士准入+校本教师教育", EducationStage: "综合学校"}))
	must(t, s.RegisterCountry(curriculum.Country{Code: "BR", Name: "巴西", SystemContext: "分散立法+州级差异", EducationStage: "基础教育"}))

	must(t, s.RegisterInstitution(curriculum.Institution{ID: "cn-u", CountryCode: "CN", Name: "华东示范师范大学"}))
	must(t, s.RegisterInstitution(curriculum.Institution{ID: "de-u", CountryCode: "DE", Name: "慕尼黑师范大学"}))
	must(t, s.RegisterInstitution(curriculum.Institution{ID: "fi-u", CountryCode: "FI", Name: "赫尔辛基大学"}))

	must(t, s.RecordFramework(curriculum.CompetencyFramework{
		ID: "cn-fw-1", CountryCode: "CN", Version: "v1", Name: "师范生能力框架",
		Items:  []curriculum.CompetencyItem{{Code: "T7", Description: "运用智能技术改进教学"}},
		Source: curriculum.SourceReference{Title: "国家师范生能力标准", Kind: "regulation"},
	}))
	must(t, s.RecordFramework(curriculum.CompetencyFramework{
		ID: "de-fw-1", CountryCode: "DE", Version: "2024", Name: "教师教育标准",
		Items:  []curriculum.CompetencyItem{{Code: "K4", Description: "Medienkompetenz und Digitalität"}},
		Source: curriculum.SourceReference{Title: "KMQ 标准", Kind: "regulation"},
	}))

	src := func(title string) []curriculum.SourceReference {
		return []curriculum.SourceReference{{Title: title, Kind: "official_catalogue", URL: "https://example.edu/" + title}}
	}

	must(t, s.RecordCourse(curriculum.Course{
		ID: "cn-c1", InstitutionID: "cn-u", ProgramVersion: "2026-A", LocalCode: "EDU-AI-01",
		NameLocal: "人工智能教育基础", Credits: 2, CreditSystem: "credits",
		TopicTags:       []string{"ai-in-education"},
		DeliveryByTopic: map[string]curriculum.DeliveryMode{"ai-in-education": curriculum.DeliveryStandalone},
		Sources:         src("cn-catalogue-2026"), EffectiveFrom: "2026-09-01",
	}))
	must(t, s.RecordCourse(curriculum.Course{
		ID: "de-c1", InstitutionID: "de-u", ProgramVersion: "2025WS", LocalCode: "FDD-210",
		NameLocal: "Fachdidaktik mit digitalen Werkzeugen", NameTranslated: "融入数字工具的学科教学法",
		Credits: 6, CreditSystem: "ECTS",
		TopicTags:       []string{"ai-in-education"},
		DeliveryByTopic: map[string]curriculum.DeliveryMode{"ai-in-education": curriculum.DeliveryEmbedded},
		Practice:        []curriculum.PracticeComponent{{Type: "school_placement", DurationWeeks: 15, Required: true}},
		Sources:         src("de-module-handbook"),
	}))
	must(t, s.RecordCourse(curriculum.Course{
		ID: "fi-c1", InstitutionID: "fi-u", ProgramVersion: "2024-26", LocalCode: "EDU301",
		NameLocal: "AI in Teaching", Credits: 5, CreditSystem: "ECTS",
		TopicTags:       []string{"ai-in-education"},
		DeliveryByTopic: map[string]curriculum.DeliveryMode{"ai-in-education": curriculum.DeliveryStandalone},
		Sources:         src("fi-opsu"),
	}))
	must(t, s.RecordCourse(curriculum.Course{
		ID: "fi-c2", InstitutionID: "fi-u", ProgramVersion: "2024-26", LocalCode: "EDU120",
		NameLocal: "Pedagogical Designs", Credits: 5, CreditSystem: "ECTS",
		TopicTags:       []string{"ai-in-education"},
		DeliveryByTopic: map[string]curriculum.DeliveryMode{"ai-in-education": curriculum.DeliveryEmbedded},
		Sources:         src("fi-opsu"),
	}))
	return s
}

func aiMapping(country, expert string, confidence float64) curriculum.MappingInput {
	return curriculum.MappingInput{
		Concept:      "ai-in-education",
		Anchor:       curriculum.ConceptAnchor{CountryCode: country, Kind: "course_topic", Ref: "ai-in-education"},
		Relationship: "equivalent",
		Expert:       expert, Confidence: confidence,
		Basis: "对照原始课程目录的主题与开设方式",
		Scope: curriculum.MappingScope{EducationStage: "secondary"},
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
}

func TestUnapprovedMappingExcludedFromComparison(t *testing.T) {
	s := seedWorld(t)
	_, err := s.SubmitMapping(aiMapping("CN", "张专家", 0.8))
	must(t, err)

	// 全部映射尚未审批：禁止产生正式比较，避免虚假多数。
	if _, err := s.CompareConcept("ai-in-education"); !errors.Is(err, curriculum.ErrState) {
		t.Fatalf("无已批准映射时应返回 ErrState，得到 %v", err)
	}
}

func TestApprovalGateAndDeliveryVote(t *testing.T) {
	s := seedWorld(t)

	mCN, err := s.SubmitMapping(aiMapping("CN", "张专家", 0.85))
	must(t, err)
	mDE, err := s.SubmitMapping(aiMapping("DE", "Müller 专家", 0.7))
	must(t, err)
	mFI, err := s.SubmitMapping(aiMapping("FI", "Virtanen 专家", 0.6))
	must(t, err)

	// 未批准的映射不能进入口径。
	_, err = s.CreateCaliber(curriculum.CaliberInput{
		ID: "cal-1", Label: "第一口径", ApprovedMappingIDs: []string{mCN.ID},
	})
	if !errors.Is(err, curriculum.ErrInvalid) {
		t.Fatalf("口径收录待审批映射应失败，得到 %v", err)
	}

	for _, id := range []string{mCN.ID, mDE.ID, mFI.ID} {
		_, err = s.ReviewMapping(id, curriculum.MappingReview{Reviewer: "评审委员会", Approve: true})
		must(t, err)
	}
	// FI 的映射存在未解决异议，比较时必须显式标注。
	_, err = s.AddObjection(mFI.ID, curriculum.Objection{Expert: "李专家", Reason: "芬兰两课主题边界与规范概念不完全一致"})
	must(t, err)

	cmp, err := s.CompareConcept("ai-in-education")
	must(t, err)

	if len(cmp.Comparable) != 3 {
		t.Fatalf("应有 3 国可比证据，得到 %d", len(cmp.Comparable))
	}
	// BR 从未提交获批映射 → 显式列入尚无共识，而不是被静默忽略。
	if len(cmp.NoConsensusCountries) != 1 || cmp.NoConsensusCountries[0] != "BR" {
		t.Fatalf("无共识国家应为 [BR]，得到 %v", cmp.NoConsensusCountries)
	}
	// 按国家计票：CN standalone、DE embedded、FI mixed（同时存在两种开设方式）。
	if cmp.DeliveryVote["standalone"] != 1 || cmp.DeliveryVote["embedded"] != 1 || cmp.DeliveryVote["mixed"] != 1 {
		t.Fatalf("开设方式投票错误: %v", cmp.DeliveryVote)
	}
	var fiObjections int
	for _, ev := range cmp.Comparable {
		if ev.CountryCode == "FI" {
			fiObjections = len(ev.UnresolvedObjections)
		}
	}
	if fiObjections != 1 {
		t.Fatalf("FI 证据应携带 1 条未解决异议，得到 %d", fiObjections)
	}
}

func TestCaliberRejectsRejectedMapping(t *testing.T) {
	s := seedWorld(t)
	m, err := s.SubmitMapping(aiMapping("CN", "张专家", 0.85))
	must(t, err)
	_, err = s.ReviewMapping(m.ID, curriculum.MappingReview{Reviewer: "委员会", Approve: false, Note: "概念不等价"})
	must(t, err)
	_, err = s.CreateCaliber(curriculum.CaliberInput{
		ID: "cal-x", Label: "x", ApprovedMappingIDs: []string{m.ID},
	})
	if !errors.Is(err, curriculum.ErrInvalid) {
		t.Fatalf("驳回映射进入口径应失败，得到 %v", err)
	}
}

func TestStageBlockersAndExplicitConclusion(t *testing.T) {
	s := seededPilot(t)
	pilot := "p1"
	cal := sCaliber(s)

	// 阶段一：样本缺失却试图下成效结论 → 拒绝，系统绝不自动生成结论。
	bad := curriculum.StageInput{
		ID: "s-blocked", Label: "秋季中期",
		Results: []curriculum.IndicatorResult{
			{MetricCode: "m_pass", SampleSize: 80, ExpectedSample: 120, MissingReason: "两所合作校未交回", DefinitionFingerprint: cal.MetricFingerprint("m_pass")},
			{MetricCode: "m_ret", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.91), DefinitionFingerprint: cal.MetricFingerprint("m_ret")},
		},
		Conclusion: &curriculum.OutcomeConclusion{RecordedBy: "专家组", Decision: "continue", Rationale: "趋势良好"},
	}
	if _, err := s.AddStage(pilot, bad); !errors.Is(err, curriculum.ErrState) {
		t.Fatalf("存在样本缺失阻断时记录结论应失败，得到 %v", err)
	}

	// 同一阶段不带结论可以入库，阻断项被如实保留。
	bad.Conclusion = nil
	st, err := s.AddStage(pilot, bad)
	must(t, err)
	if len(st.Blockers) != 1 || !strings.Contains(st.Blockers[0], "m_pass") {
		t.Fatalf("应保留 m_pass 的样本缺失阻断，得到 %v", st.Blockers)
	}

	// 阶段二：定义指纹与口径不一致 → 阻断。
	drift := curriculum.StageInput{
		ID: "s-drift", Label: "冬季",
		Results: []curriculum.IndicatorResult{
			{MetricCode: "m_pass", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.78), DefinitionFingerprint: "olddef000000"},
			{MetricCode: "m_ret", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.9), DefinitionFingerprint: cal.MetricFingerprint("m_ret")},
		},
	}
	st2, err := s.AddStage(pilot, drift)
	must(t, err)
	if len(st2.Blockers) != 1 || !strings.Contains(st2.Blockers[0], "统计定义已改变") {
		t.Fatalf("应报告定义漂移阻断，得到 %v", st2.Blockers)
	}

	// 阶段三：样本完整、口径一致，允许专家显式结论。
	good := curriculum.StageInput{
		ID: "s-good", Label: "春季终期",
		Results: []curriculum.IndicatorResult{
			{MetricCode: "m_pass", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.82), DefinitionFingerprint: cal.MetricFingerprint("m_pass")},
			{MetricCode: "m_ret", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.93), DefinitionFingerprint: cal.MetricFingerprint("m_ret")},
		},
		Conclusion: &curriculum.OutcomeConclusion{RecordedBy: "专家组", Decision: "continue", Rationale: "两指标均达标且样本完整"},
	}
	st3, err := s.AddStage(pilot, good)
	must(t, err)
	if st3.Conclusion == nil || st3.Conclusion.Decision != "continue" {
		t.Fatalf("完整阶段应允许显式 continue 结论")
	}
}

func TestRevisionKeepsOldVersionSupportingRecommendation(t *testing.T) {
	s := seededPilot(t)

	// 用完整阶段 s-good 支撑一条政策建议，精确引用映射 M001（CN 已批准版）。
	goodStage(t, s, "s-good")
	rec, err := s.AddRecommendation(curriculum.RecommendationInput{
		ID: "r1", Title: "将 AI 主题纳入核心模块", Detail: "依据中德可比证据与完整试点数据",
		PilotID: "p1", StageIDs: []string{"s-good"}, MappingIDs: []string{"M001"},
	})
	must(t, err)

	// 专家随后修订映射：M001 被替代，M004 重新走审批。
	mNew, err := s.ReviseMapping("M001", aiMapping("CN", "张专家（修订）", 0.66))
	must(t, err)
	if mNew.RevisionOf != "M001" || mNew.Status != curriculum.MappingSubmitted {
		t.Fatalf("修订版应从 submitted 重新审批并指向旧版，得到 %+v", mNew)
	}
	old, err := s.GetMapping("M001")
	must(t, err)
	if old.Status != curriculum.MappingSuperseded {
		t.Fatalf("旧版状态应为 superseded，得到 %s", old.Status)
	}

	// 旧政策建议不被追溯删除，仍精确指向曾支撑它的旧版。
	if rec.MappingIDs[0] != "M001" {
		t.Fatalf("建议应永久保留旧映射版本引用")
	}

	pp, err := s.AddProposal(curriculum.ProposalInput{
		ID: "pp1", Title: "2027 培养方案调整", Description: "将 AI 主题调整为核心模块",
		ProposedBy: "决策者", RecommendationIDs: []string{"r1"},
	})
	must(t, err)

	chain, err := s.EvidenceChainFor(pp.ID)
	must(t, err)
	if chain.Caliber.ID != "cal-1" {
		t.Fatalf("证据链应绑定口径 cal-1，得到 %s", chain.Caliber.ID)
	}
	var foundOld bool
	for _, r := range chain.Recommendations {
		for _, m := range r.Mappings {
			if m.ID == "M001" && m.Status == curriculum.MappingSuperseded {
				foundOld = true
			}
		}
	}
	if !foundOld {
		t.Fatal("证据链必须展示被修订替代但曾支撑建议的旧版映射")
	}
	if len(chain.Comparisons) != 1 {
		t.Fatalf("应包含 ai-in-education 的正式比较，得到 %d 个", len(chain.Comparisons))
	}
	if len(chain.Decisions) != 1 || chain.Decisions[0] != "s-good: continue" {
		t.Fatalf("应汇总显式阶段决策，得到 %v", chain.Decisions)
	}
}

func TestRecommendationRequiresConcludedStageAndCaliberMapping(t *testing.T) {
	s := seededPilot(t)
	goodStage(t, s, "s-good")

	// 引用未进入口径的 FI 映射（M003）应被拒绝。
	_, err := s.AddRecommendation(curriculum.RecommendationInput{
		ID: "r-bad", Title: "x", Detail: "y", PilotID: "p1",
		StageIDs: []string{"s-good"}, MappingIDs: []string{"M003"},
	})
	if !errors.Is(err, curriculum.ErrInvalid) {
		t.Fatalf("引用口径外映射应失败，得到 %v", err)
	}
}

func TestEvidenceChainShowsBlockedStagesAndNoConsensus(t *testing.T) {
	s := seededPilot(t)
	goodStage(t, s, "s-good")
	// 再录一个带阻断、无结论的阶段。
	_, err := s.AddStage("p1", curriculum.StageInput{
		ID: "s-late", Label: "追踪",
		Results: []curriculum.IndicatorResult{
			{MetricCode: "m_pass", SampleSize: 10, ExpectedSample: 120, MissingReason: "随访流失", DefinitionFingerprint: sCaliber(s).MetricFingerprint("m_pass")},
			{MetricCode: "m_ret", SampleSize: 10, ExpectedSample: 120, MissingReason: "随访流失", DefinitionFingerprint: sCaliber(s).MetricFingerprint("m_ret")},
		},
	})
	must(t, err)
	_, err = s.AddRecommendation(curriculum.RecommendationInput{
		ID: "r1", Title: "t", Detail: "d", PilotID: "p1",
		StageIDs: []string{"s-good"}, MappingIDs: []string{"M001"},
	})
	must(t, err)
	_, err = s.AddProposal(curriculum.ProposalInput{
		ID: "pp1", Title: "调整", Description: "d", ProposedBy: "决策者",
		RecommendationIDs: []string{"r1"},
	})
	must(t, err)

	chain, err := s.EvidenceChainFor("pp1")
	must(t, err)
	// 证据链锚定口径 cal-1（仅含 CN/DE 映射）：两国可比；
	// FI 映射虽已批准但不在该口径，BR 从未获批 —— 两者都显式列入无共识。
	var cmp *curriculum.ConceptComparison
	for i := range chain.Comparisons {
		if chain.Comparisons[i].Concept == "ai-in-education" {
			cmp = &chain.Comparisons[i]
		}
	}
	if cmp == nil {
		t.Fatal("证据链缺少 ai-in-education 比较")
	}
	if len(cmp.Comparable) != 2 {
		t.Fatalf("口径内应有 2 国可比证据，得到 %d", len(cmp.Comparable))
	}
	if len(cmp.NoConsensusCountries) != 2 ||
		cmp.NoConsensusCountries[0] != "BR" || cmp.NoConsensusCountries[1] != "FI" {
		t.Fatalf("无共识国家应为 [BR FI]，得到 %v", cmp.NoConsensusCountries)
	}
	// 被引用的建议只挂已结论阶段；阻断阶段不进入任何建议，证据链也不伪造其决策。
	if len(chain.Decisions) != 1 || chain.Decisions[0] != "s-good: continue" {
		t.Fatalf("决策汇总应只含已结论阶段，得到 %v", chain.Decisions)
	}
	// 但同一试点未被引用的阻断阶段 s-late 仍须独立列出，提示数据尚不足以支持其他判断。
	if len(chain.BlockedStages) != 1 || chain.BlockedStages[0].ID != "s-late" {
		t.Fatalf("证据链应暴露阻断阶段 s-late，得到 %+v", chain.BlockedStages)
	}
	if chain.BlockedStages[0].Conclusion != nil {
		t.Fatal("阻断阶段不应携带成效结论")
	}
}

// TestProposalRejectsMixedCalibers 验证一次课程调整不能混用不同口径的建议。
func TestProposalRejectsMixedCalibers(t *testing.T) {
	s := seededPilot(t)
	goodStage(t, s, "s-good")
	_, err := s.AddRecommendation(curriculum.RecommendationInput{
		ID: "r1", Title: "t", Detail: "d", PilotID: "p1",
		StageIDs: []string{"s-good"}, MappingIDs: []string{"M001"},
	})
	must(t, err)

	// 建立第二口径（仅 DE 映射）与第二个试点、完整阶段。
	cal2, err := s.CreateCaliber(curriculum.CaliberInput{
		ID: "cal-2", Label: "第二口径", ApprovedMappingIDs: []string{"M002"},
		Metrics: []curriculum.MetricDefinition{
			{Code: "m_pass", Name: "通过率", Definition: "合格/参加", Unit: "比例"},
		},
	})
	must(t, err)
	_, err = s.CreatePilot(curriculum.PilotInput{
		ID: "p2", Name: "试点二", CaliberID: cal2.ID, Plan: "另一方案",
		InstitutionIDs: []string{"de-u"},
		Cohorts:        []curriculum.TeacherCohort{{Label: "教师", Size: 10}},
		Indicators:     []string{"m_pass"},
	})
	must(t, err)
	_, err = s.AddStage("p2", curriculum.StageInput{
		ID: "s2b", Label: "终期",
		Results: []curriculum.IndicatorResult{
			{MetricCode: "m_pass", SampleSize: 10, ExpectedSample: 10, Value: ptr(0.7),
				DefinitionFingerprint: cal2.MetricFingerprint("m_pass")},
		},
		Conclusion: &curriculum.OutcomeConclusion{RecordedBy: "组", Decision: "stop", Rationale: "不达标"},
	})
	must(t, err)
	_, err = s.AddRecommendation(curriculum.RecommendationInput{
		ID: "r2", Title: "t2", Detail: "d2", PilotID: "p2",
		StageIDs: []string{"s2b"}, MappingIDs: []string{"M002"},
	})
	must(t, err)

	_, err = s.AddProposal(curriculum.ProposalInput{
		ID: "pp-mix", Title: "混用口径提案", Description: "d", ProposedBy: "决策者",
		RecommendationIDs: []string{"r1", "r2"},
	})
	if !errors.Is(err, curriculum.ErrInvalid) {
		t.Fatalf("混用口径的提案应被拒绝，得到 %v", err)
	}
}

// TestImmutability 验证原始课程与口径一经登记不可覆盖。
func TestImmutability(t *testing.T) {
	s := seedWorld(t)
	c, err := s.GetCourse("cn-c1")
	must(t, err)
	c.Credits = 99
	if err := s.RecordCourse(c); !errors.Is(err, curriculum.ErrConflict) {
		t.Fatalf("重复登记同一课程 ID 应冲突，得到 %v", err)
	}
}

// 以下为辅助函数与种子构造。

func ptr(v float64) *float64 { return &v }

func sCaliber(s *curriculum.Service) curriculum.CaliberVersion {
	cv, _ := s.GetCaliber("cal-1")
	return cv
}

// seededPilot 构造：四国世界 + CN/DE 映射获批进入口径 cal-1 + 试点 p1。
func seededPilot(t *testing.T) *curriculum.Service {
	t.Helper()
	s := seedWorld(t)
	mCN, err := s.SubmitMapping(aiMapping("CN", "张专家", 0.85))
	must(t, err) // M001
	mDE, err := s.SubmitMapping(aiMapping("DE", "Müller", 0.7))
	must(t, err) // M002
	mFI, err := s.SubmitMapping(aiMapping("FI", "Virtanen", 0.6))
	must(t, err) // M003
	_, err = s.ReviewMapping(mCN.ID, curriculum.MappingReview{Reviewer: "委员会", Approve: true})
	must(t, err)
	_, err = s.ReviewMapping(mDE.ID, curriculum.MappingReview{Reviewer: "委员会", Approve: true})
	must(t, err)
	_, err = s.ReviewMapping(mFI.ID, curriculum.MappingReview{Reviewer: "委员会", Approve: true})
	must(t, err)

	cal, err := s.CreateCaliber(curriculum.CaliberInput{
		ID: "cal-1", Label: "第一口径",
		ApprovedMappingIDs: []string{mCN.ID, mDE.ID},
		Metrics: []curriculum.MetricDefinition{
			{Code: "m_pass", Name: "考核通过率", Definition: "终期考核合格人数/参加人数", Unit: "比例"},
			{Code: "m_ret", Name: "留任率", Definition: "入职一年后仍在岗人数/入职人数", Unit: "比例"},
		},
	})
	must(t, err)
	_ = cal

	_, err = s.CreatePilot(curriculum.PilotInput{
		ID: "p1", Name: "AI 融入教学法试点", CaliberID: "cal-1",
		Plan:           "在中德各两所合作校推行融入式工作坊",
		InstitutionIDs: []string{"cn-u", "de-u"},
		Cohorts:        []curriculum.TeacherCohort{{Label: "在职初中教师", Size: 120}},
		Indicators:     []string{"m_pass", "m_ret"},
	})
	must(t, err)
	return s
}

func goodStage(t *testing.T, s *curriculum.Service, id string) {
	t.Helper()
	cal := sCaliber(s)
	_, err := s.AddStage("p1", curriculum.StageInput{
		ID: id, Label: "终期",
		Results: []curriculum.IndicatorResult{
			{MetricCode: "m_pass", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.82), DefinitionFingerprint: cal.MetricFingerprint("m_pass")},
			{MetricCode: "m_ret", SampleSize: 120, ExpectedSample: 120, Value: ptr(0.93), DefinitionFingerprint: cal.MetricFingerprint("m_ret")},
		},
		Conclusion: &curriculum.OutcomeConclusion{RecordedBy: "专家组", Decision: "continue", Rationale: "样本完整且达标"},
	})
	must(t, err)
}
