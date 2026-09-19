package evidence

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// setup 构造三个制度、两套框架、两所院校、两门 AI 开设方式不同的课程与一个 CPD 项目。
func setup(t *testing.T) *Service {
	t.Helper()
	svc := NewService(NewTestStore(t))

	must(t, svc.AddSystem(SystemContext{ID: "SYS-CN", Country: "中国", Description: "师范院校课程方案"}))
	must(t, svc.AddSystem(SystemContext{ID: "SYS-FI", Country: "芬兰", Description: "研究型硕士培养"}))
	must(t, svc.AddSystem(SystemContext{ID: "SYS-JP", Country: "日本", Description: "两阶段执照制"}))

	must(t, svc.AddSource(Source{ID: "SRC-1", Citation: "OECD 2024 示例章节"}))
	must(t, svc.AddSource(Source{ID: "SRC-2", Citation: "示例师范院校培养方案"}))

	must(t, svc.AddFramework(Framework{
		ID: "FW-CN-1", SystemID: "SYS-CN", Name: "中方素养框架", Version: "1",
		Domains: []FrameworkDomain{
			{Key: "tech", Name: "数字素养与 AI 应用"},
			{Key: "practice", Name: "教育教学实践"},
		},
	}))
	must(t, svc.AddFramework(Framework{
		ID: "FW-FI-1", SystemID: "SYS-FI", Name: "Finnish competence map", Version: "1",
		Domains: []FrameworkDomain{{Key: "digital", Name: "Digital competence"}},
	}))

	must(t, svc.AddInstitution(Institution{ID: "INST-CN", Name: "中方示例师大", SystemID: "SYS-CN"}))
	must(t, svc.AddInstitution(Institution{ID: "INST-FI", Name: "芬方示例大学", SystemID: "SYS-FI"}))

	_, err := svc.AddCourse(AddCourseInput{
		ID: "C-CN-AI", Version: "2026", InstitutionID: "INST-CN",
		Title: "人工智能教育基础", Credits: 3, CreditUnit: "学分",
		AI:          AIDelivery{Mode: AIModeStandalone, Notes: "独立设课"},
		Practice:    PracticeComponent{Included: false},
		FrameworkID: "FW-CN-1", SourceIDs: []string{"SRC-2"}, EffectiveFrom: "2026-09-01",
	})
	must(t, err)
	_, err = svc.AddCourse(AddCourseInput{
		ID: "C-FI-DIG", Version: "2025", InstitutionID: "INST-FI",
		Title: "Digital Competence in Teaching", Credits: 5, CreditUnit: "ECTS",
		AI:          AIDelivery{Mode: AIModeIntegrated, IntegratedInto: []string{"EDU-METHOD"}, ContactHoursShare: 0.2},
		Practice:    PracticeComponent{Included: true, Weeks: 4, Hours: 60, Mentored: true},
		FrameworkID: "FW-FI-1", SourceIDs: []string{"SRC-1"}, EffectiveFrom: "2025-08-01",
	})
	must(t, err)

	must(t, svc.AddCPD(CPDProgram{
		ID: "CPD-JP-IND", SystemID: "SYS-JP", Name: "初任者研修",
		Stage: CPDInduction, Required: true, Hours: 120, SourceIDs: []string{"SRC-1"},
	}))
	must(t, svc.AddCPD(CPDProgram{
		ID: "CPD-FI-MENT", SystemID: "SYS-FI", Name: "Induction mentoring programme",
		Stage: CPDInduction, Required: false, Hours: 40, SourceIDs: []string{"SRC-1"},
	}))
	return svc
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
}

func wantErrIs(t *testing.T, err error, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("期望错误 %v，实际 %v", target, err)
	}
}

func competencyMapping(conf float64, applicability string) SubmitMappingInput {
	return SubmitMappingInput{
		Kind:          KindCompetency,
		Source:        MappingRef{SystemID: "SYS-CN", Kind: RefFrameworkDomain, RefID: "FW-CN-1#tech", Label: "数字素养"},
		Target:        MappingRef{SystemID: "SYS-FI", Kind: RefFrameworkDomain, RefID: "FW-FI-1#digital", Label: "Digital competence"},
		Confidence:    conf,
		Applicability: applicability,
		Rationale:     "职前数字素养总体要求相近",
		ExpertID:      "expert-wang",
	}
}

// 规则一：未经批准的映射不进入正式比较；驳回必须附理由。
func TestUnapprovedMappingExcludedFromComparison(t *testing.T) {
	svc := setup(t)

	id, err := svc.SubmitMapping(competencyMapping(0.7, "仅限职前总体要求"))
	must(t, err)
	if id != "M-1@1" {
		t.Fatalf("首个映射实例 ID 应为 M-1@1，实际 %s", id)
	}

	view, err := svc.Compare(KindCompetency)
	must(t, err)
	if len(view.ComparablePairs) != 0 || view.PendingCount != 1 {
		t.Fatalf("待审映射不得进入正式比较，得到 pairs=%d pending=%d", len(view.ComparablePairs), view.PendingCount)
	}

	// 驳回必须给出书面理由。
	wantErrIs(t, svc.ReviewMapping(id, "board", MappingRejected, ""), ErrObjectionMissing)
	must(t, svc.ReviewMapping(id, "board", MappingRejected, "两框架对 AI 专项要求层级不同"))
	view, _ = svc.Compare(KindCompetency)
	if len(view.ComparablePairs) != 0 || view.RejectedCount != 1 {
		t.Fatalf("驳回映射不得进入正式比较，得到 %+v", view)
	}
	// 已有结论的映射不能重复审批。
	wantErrIs(t, svc.ReviewMapping(id, "board", MappingApproved, "x"), ErrMappingNotPending)

	// 另一条获批映射才进入比较，且异议会被标记为有争议。
	id2, err := svc.SubmitMapping(competencyMapping(0.8, "职前总体数字素养"))
	must(t, err)
	must(t, svc.AddObjection(id2, "expert-li", "适用范围还应排除学时比较"))
	must(t, svc.ReviewMapping(id2, "board", MappingApproved, "同意，按声明范围使用"))
	view, _ = svc.Compare(KindCompetency)
	if len(view.ComparablePairs) != 1 {
		t.Fatalf("应恰有 1 对可比材料，实际 %d", len(view.ComparablePairs))
	}
	if !view.ComparablePairs[0].Disputed {
		t.Fatal("带异议的获批映射应在比较中标记 disputed")
	}
	if !slices.Contains(view.UncoveredSystems, "SYS-JP") {
		t.Fatal("无任何获批映射涉及的日本制度应出现在 uncovered_systems")
	}
}

// 课程名称相同不构成可比：同名课程在不同制度下 AI 开设方式不同，
// 在获批课程级映射存在之前，course 比较必须为空。
func TestSameCourseNameDoesNotCreateComparability(t *testing.T) {
	svc := setup(t)

	view, err := svc.Compare(KindCourse)
	must(t, err)
	if len(view.ComparablePairs) != 0 {
		t.Fatalf("没有获批课程映射前不应有可比对，实际 %d", len(view.ComparablePairs))
	}

	// 独立设课与融入式课程之间的课程级映射即便提交，也默认不进入比较。
	id, err := svc.SubmitMapping(SubmitMappingInput{
		Kind:          KindCourse,
		Source:        MappingRef{SystemID: "SYS-CN", Kind: RefCourse, RefID: "C-CN-AI@2026"},
		Target:        MappingRef{SystemID: "SYS-FI", Kind: RefCourse, RefID: "C-FI-DIG@2025"},
		Confidence:    0.4,
		Applicability: "仅课程定位参照，不得用于统计独立设课比例",
		ExpertID:      "expert-li",
	})
	must(t, err)
	view, _ = svc.Compare(KindCourse)
	if len(view.ComparablePairs) != 0 || view.PendingCount != 1 {
		t.Fatalf("待审的 AI 课程映射不得制造多数意见，得到 %+v", view)
	}
	must(t, svc.ReviewMapping(id, "board", MappingApproved, "仅限定位参照"))
	view, _ = svc.Compare(KindCourse)
	if len(view.ComparablePairs) != 1 {
		t.Fatalf("获批后应有 1 对可比材料，实际 %d", len(view.ComparablePairs))
	}
}

// CPD 与职前课程分属不同比较类别，不能混为一谈。
func TestCPDComparedSeparatelyFromCourses(t *testing.T) {
	svc := setup(t)

	// CPD 类映射两端都必须是 CPD 对象；把 CPD 映到素养领域应被拒绝。
	_, err := svc.SubmitMapping(SubmitMappingInput{
		Kind:          KindCPD,
		Source:        MappingRef{SystemID: "SYS-JP", Kind: RefCPD, RefID: "CPD-JP-IND"},
		Target:        MappingRef{SystemID: "SYS-FI", Kind: RefFrameworkDomain, RefID: "FW-FI-1#digital"},
		Confidence:    0.5,
		Applicability: "仅限入职阶段数字研修",
		ExpertID:      "expert-zhao",
	})
	wantErrIs(t, err, ErrInvalid)

	// 同类 CPD 映射（入职研修 ↔ 入职辅导）可以提交，但在获批前同样不进入任何比较。
	id, err := svc.SubmitMapping(SubmitMappingInput{
		Kind:          KindCPD,
		Source:        MappingRef{SystemID: "SYS-JP", Kind: RefCPD, RefID: "CPD-JP-IND"},
		Target:        MappingRef{SystemID: "SYS-FI", Kind: RefCPD, RefID: "CPD-FI-MENT"},
		Confidence:    0.5,
		Applicability: "仅限入职阶段的制度化辅导比较",
		ExpertID:      "expert-zhao",
	})
	must(t, err)

	courseView, err := svc.Compare(KindCourse)
	must(t, err)
	cpdView, err := svc.Compare(KindCPD)
	must(t, err)
	if len(courseView.ComparablePairs) != 0 {
		t.Fatal("CPD 映射不得出现在课程比较中")
	}
	if len(cpdView.ComparablePairs) != 0 || cpdView.PendingCount != 1 {
		t.Fatalf("待审 CPD 映射只应计入待审，实际 %+v", cpdView)
	}
	must(t, svc.ReviewMapping(id, "board", MappingApproved, "仅限入职辅导比较"))
	cpdView, _ = svc.Compare(KindCPD)
	if len(cpdView.ComparablePairs) != 1 {
		t.Fatalf("获批后 CPD 比较应有 1 对，实际 %d", len(cpdView.ComparablePairs))
	}
}

// 映射两端必须属于不同制度。
func TestMappingRequiresDifferentSystems(t *testing.T) {
	svc := setup(t)
	in := competencyMapping(0.7, "x")
	in.Target.SystemID = "SYS-CN"
	in.Target.RefID = "FW-CN-1#practice"
	_, err := svc.SubmitMapping(in)
	wantErrIs(t, err, ErrSystemMismatch)
}

// 规则二：映射修订后旧版曾支撑过的政策建议仍可追溯，且新版必须重新审批。
func TestRevisionLineageAndRecommendationTrace(t *testing.T) {
	svc := setup(t)

	id, err := svc.SubmitMapping(competencyMapping(0.72, "职前数字素养总体要求"))
	must(t, err)
	must(t, svc.ReviewMapping(id, "board", MappingApproved, "通过"))

	rec, err := svc.CreateRecommendation(CreateRecommendationInput{
		ID: "REC-1", Title: "数字素养对齐可用于总体政策对话", MappingIDs: []string{id},
	})
	must(t, err)
	if len(rec.Basis.MappingSnapshot) != 1 || rec.Basis.MappingSnapshot[0].ID != "M-1@1" {
		t.Fatal("政策建议必须冻结获批映射的完整快照")
	}

	// 提交修订：旧版变 superseded，新版为 proposed。
	id2, err := svc.ReviseMapping(id, "expert-wang", SubmitMappingInput{
		Confidence: 0.55, Applicability: "收窄：排除 AI 专项学时", Rationale: "新资料显示芬方 AI 仅占两成",
	})
	must(t, err)
	if id2 != "M-1@2" {
		t.Fatalf("修订版实例 ID 应为 M-1@2，实际 %s", id2)
	}

	lineage, err := svc.MappingLineage("M-1")
	must(t, err)
	if len(lineage) != 2 || lineage[0].Status != MappingSuperseded || lineage[1].Status != MappingProposed {
		t.Fatalf("谱系状态不符: %+v", lineage)
	}

	// 旧版不再获批：正式比较暂时为空，且不能用旧版再立建议。
	view, _ := svc.Compare(KindCompetency)
	if len(view.ComparablePairs) != 0 {
		t.Fatal("旧版被替代、新版未批期间，不应有可比材料")
	}
	_, err = svc.CreateRecommendation(CreateRecommendationInput{ID: "REC-X", Title: "x", MappingIDs: []string{"M-1@1"}})
	wantErrIs(t, err, ErrMappingNotApproved)

	// 但旧版曾支撑过 REC-1 这一事实永不丢失。
	backed := svc.BackedRecommendations("M-1@1")
	if len(backed) != 1 || backed[0].ID != "REC-1" {
		t.Fatalf("应能沿旧映射版本回溯到 REC-1，实际 %+v", backed)
	}

	// 新旧检查：REC-1 依据的版本已落后于当前最新版本。
	fresh, err := svc.CheckRecommendationFreshness("REC-1")
	must(t, err)
	if fresh.AllCurrent || fresh.Entries[0].LatestVersion != 2 || fresh.Entries[0].SnapshotVersion != 1 {
		t.Fatalf("新鲜度检查应提示建议依据已被修订: %+v", fresh)
	}

	// 新版获批后正式比较恢复，并可立新建议。
	must(t, svc.ReviewMapping(id2, "board", MappingApproved, "按收窄范围通过"))
	view, _ = svc.Compare(KindCompetency)
	if len(view.ComparablePairs) != 1 || view.ComparablePairs[0].MappingID != "M-1@2" {
		t.Fatalf("正式比较只能包含新版 M-1@2，实际 %+v", view.ComparablePairs)
	}
	_, err = svc.CreateRecommendation(CreateRecommendationInput{ID: "REC-2", Title: "按收窄范围对齐", MappingIDs: []string{"M-1@2"}})
	must(t, err)
}

func calibrationInput() RegisterCalibrationInput {
	return RegisterCalibrationInput{
		LogicalID:      "CAL-1",
		CurriculumRefs: map[string]string{"INST-CN": "C-CN-AI@2026"},
		FrameworkRefs:  map[string]string{"INST-CN": "FW-CN-1"},
		Metrics: []MetricDefinition{
			{Key: "lesson_design", Name: "AI 融合教案评分", Definition: "统一量规双盲评分 0-100", Unit: "分", HigherBetter: true},
			{Key: "retention", Name: "留任意向", Definition: "匿名问卷 5 点均值", Unit: "均值"},
		},
		ExpectedInstitutions: []string{"INST-CN"},
		ExpectedCohorts:      []string{"物理师范生", "语文师范生"},
		MinResponseRate:      0.8,
	}
}

// 规则三：样本缺失、应答率不足、指标定义改变或口径变更时，不得生成成效结论。
func TestPilotCalibrationGates(t *testing.T) {
	svc := setup(t)

	cal, err := svc.RegisterCalibration(calibrationInput())
	must(t, err)
	if cal.ID != "CAL-1@1" {
		t.Fatalf("口径实例 ID 应为 CAL-1@1，实际 %s", cal.ID)
	}

	// 试点指标必须取自口径定义。
	_, err = svc.CreatePilot(CreatePilotInput{
		ID: "PILOT-1", Name: "AI 教学法试点", PlanName: "2026 秋方案", CalibrationID: cal.ID,
		ParticipatingInstitutions: []string{"INST-CN"},
		TeacherCohorts:            []string{"物理师范生", "语文师范生"},
		Indicators:                []string{"lesson_design", "self_invented"},
	})
	wantErrIs(t, err, ErrDefinitionChanged)

	_, err = svc.CreatePilot(CreatePilotInput{
		ID: "PILOT-1", Name: "AI 教学法试点", PlanName: "2026 秋方案", CalibrationID: cal.ID,
		ParticipatingInstitutions: []string{"INST-CN"},
		TeacherCohorts:            []string{"物理师范生", "语文师范生"},
		Indicators:                []string{"lesson_design", "retention"},
	})
	must(t, err)

	complete := ReportStageInput{
		PilotID: "PILOT-1", Stage: "phase-1", Period: "2026-09~2027-01",
		PresentIndicators: []string{"lesson_design", "retention"},
		Observations:      map[string]float64{"lesson_design": 82.5, "retention": 4.1},
		ResponseRates:     map[string]float64{"INST-CN": 0.91},
	}

	// 闸门 1：院校样本缺失 → 拒绝。
	bad := complete
	bad.MissingInstitutions = []string{"INST-CN"}
	_, err = svc.ReportStage(bad)
	wantErrIs(t, err, ErrSampleIncomplete)

	// 闸门 2：教师群体缺失 → 拒绝。
	bad = complete
	bad.MissingCohorts = []string{"语文师范生"}
	_, err = svc.ReportStage(bad)
	wantErrIs(t, err, ErrSampleIncomplete)

	// 闸门 3：应答率低于口径下限 → 拒绝。
	bad = complete
	bad.ResponseRates = map[string]float64{"INST-CN": 0.55}
	_, err = svc.ReportStage(bad)
	wantErrIs(t, err, ErrSampleIncomplete)

	// 闸门 4：缺少口径内指标的定义与数据 → 拒绝。
	bad = complete
	bad.PresentIndicators = []string{"lesson_design"}
	bad.Observations = map[string]float64{"lesson_design": 82.5}
	_, err = svc.ReportStage(bad)
	wantErrIs(t, err, ErrDefinitionChanged)

	// 闸门 5：观察值夹带口径外指标 → 拒绝。
	bad = complete
	bad.Observations = map[string]float64{"lesson_design": 82.5, "retention": 4.1, "rogue": 99}
	_, err = svc.ReportStage(bad)
	wantErrIs(t, err, ErrIndicatorNotInScope)

	// 数据未过闸门前不得下结论。
	_, err = svc.RecordConclusion(RecordConclusionInput{PilotID: "PILOT-1", Stage: "phase-1", Decision: "continue", Author: "g"})
	wantErrIs(t, err, ErrSampleIncomplete)

	// 完整数据通过，随后才能显式登记结论（系统本身从不自动生成）。
	if _, err := svc.ReportStage(complete); err != nil {
		t.Fatalf("完整上报应通过: %v", err)
	}
	if _, err := svc.RecordConclusion(RecordConclusionInput{
		PilotID: "PILOT-1", Stage: "phase-1", Decision: "continue", Author: "decision-group",
	}); err != nil {
		t.Fatalf("闸门通过后应能登记结论: %v", err)
	}
	pilot, _ := svc.GetPilot("PILOT-1")
	if got := pilot.Stages["phase-1"].ConclusionCalibrationVer; got != 1 {
		t.Fatalf("结论必须记录所依据的口径版本，实际 %d", got)
	}

	// 同一阶段不能重复下结论。
	_, err = svc.RecordConclusion(RecordConclusionInput{PilotID: "PILOT-1", Stage: "phase-1", Decision: "stop", Author: "g"})
	wantErrIs(t, err, ErrAlreadyExists)

	// 口径修订（v2）后，旧口径试点下的上报与结论一律阻断，杜绝定义漂移下的自动成效。
	next := calibrationInput()
	next.Metrics[0].Definition = "统一量规双盲评分 0-10（尺度变更）"
	cal2, err := svc.RegisterCalibration(next)
	must(t, err)
	if cal2.Version != 2 {
		t.Fatalf("同逻辑口径重登记应为 v2，实际 %d", cal2.Version)
	}
	phase2 := complete
	phase2.Stage = "phase-2"
	_, err = svc.ReportStage(phase2)
	wantErrIs(t, err, ErrCalibrationChanged)
	// 口径变更后，即便阶段已有结论，任何结论操作也一律被口径闸门拦下。
	_, err = svc.RecordConclusion(RecordConclusionInput{PilotID: "PILOT-1", Stage: "phase-1", Decision: "adjust", Author: "g"})
	wantErrIs(t, err, ErrCalibrationChanged)
}

// 规则四：课程调整沿证据链呈现可比材料、无共识缺口与试点结论依据。
func TestEvidenceChain(t *testing.T) {
	svc := setup(t)

	id, err := svc.SubmitMapping(competencyMapping(0.72, "职前数字素养总体要求"))
	must(t, err)
	must(t, svc.ReviewMapping(id, "board", MappingApproved, "通过"))
	_, err = svc.CreateRecommendation(CreateRecommendationInput{
		ID: "REC-1", Title: "总体数字素养可对齐", MappingIDs: []string{"M-1@1"},
	})
	must(t, err)

	// 修订映射并批准新版；REC-1 的快照仍停在 v1。
	id2, err := svc.ReviseMapping("M-1@1", "expert-wang", SubmitMappingInput{
		Confidence: 0.55, Applicability: "收窄范围", Rationale: "新材料",
	})
	must(t, err)
	must(t, svc.ReviewMapping(id2, "board", MappingApproved, "通过"))
	_, err = svc.CreateRecommendation(CreateRecommendationInput{
		ID: "REC-2", Title: "按收窄范围对齐", MappingIDs: []string{"M-1@2"},
	})
	must(t, err)

	cal, err := svc.RegisterCalibration(calibrationInput())
	must(t, err)
	_, err = svc.CreatePilot(CreatePilotInput{
		ID: "PILOT-1", Name: "AI 教学法试点", PlanName: "2026 秋方案", CalibrationID: cal.ID,
		ParticipatingInstitutions: []string{"INST-CN"},
		TeacherCohorts:            []string{"物理师范生", "语文师范生"},
		Indicators:                []string{"lesson_design", "retention"},
	})
	must(t, err)
	_, err = svc.ReportStage(ReportStageInput{
		PilotID: "PILOT-1", Stage: "phase-1", Period: "2026-09~2027-01",
		PresentIndicators: []string{"lesson_design", "retention"},
		Observations:      map[string]float64{"lesson_design": 82.5, "retention": 4.1},
		ResponseRates:     map[string]float64{"INST-CN": 0.91},
	})
	must(t, err)
	_, err = svc.RecordConclusion(RecordConclusionInput{
		PilotID: "PILOT-1", Stage: "phase-1", Decision: "continue",
		Author: "decision-group", Comment: "样本完整且方向积极，继续扩围",
	})
	must(t, err)

	_, err = svc.ProposeAdjustment(ProposeAdjustmentInput{
		ID: "ADJ-1", Title: "AI 课程改为独立课加学科融入",
		RecommendationIDs: []string{"REC-1", "REC-2"}, PilotID: "PILOT-1",
	})
	must(t, err)

	chain, err := svc.BuildEvidenceChain("ADJ-1")
	must(t, err)

	// 真正可比的只有当前仍获批的 M-1@2；旧快照 M-1@1 进入缺口而非可比清单。
	if len(chain.ComparableMaterials) != 1 || chain.ComparableMaterials[0].MappingID != "M-1@2" {
		t.Fatalf("可比材料应只含 M-1@2，实际 %+v", chain.ComparableMaterials)
	}
	foundStale := false
	for _, g := range chain.Gaps {
		if g.MappingID == "M-1@1" && strings.Contains(g.Description, "REC-1") && strings.Contains(g.Description, "M-1@2") {
			foundStale = true
		}
	}
	if !foundStale {
		t.Fatalf("缺口必须说明 M-1@1 曾支撑 REC-1 且最新为 M-1@2，实际: %+v", chain.Gaps)
	}
	if len(chain.PilotStages) != 1 || !chain.PilotStages[0].GatePassed {
		t.Fatalf("试点阶段应显示闸门通过，实际 %+v", chain.PilotStages)
	}
	if !strings.HasPrefix(chain.PilotStages[0].Conclusion, "continue") {
		t.Fatalf("应展示继续/改动/停止的结论依据，实际 %+v", chain.PilotStages[0])
	}
}

// 登记侧校验：融入式 AI 必须注明融入的课程；先修与框架必须存在。
func TestRegistrationValidation(t *testing.T) {
	svc := setup(t)

	_, err := svc.AddCourse(AddCourseInput{
		ID: "C-BAD", Version: "1", InstitutionID: "INST-CN", Title: "x", Credits: 1,
		AI: AIDelivery{Mode: AIModeIntegrated}, FrameworkID: "FW-CN-1",
	})
	wantErrIs(t, err, ErrInvalid)

	_, err = svc.AddCourse(AddCourseInput{
		ID: "C-BAD2", Version: "1", InstitutionID: "INST-CN", Title: "x", Credits: 1,
		AI: AIDelivery{Mode: AIModeStandalone}, FrameworkID: "FW-CN-1",
		Prerequisites: []string{"C-MISSING@9"},
	})
	wantErrIs(t, err, ErrMissingPrerequisite)
}
