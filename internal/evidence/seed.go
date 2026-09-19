package evidence

import "time"

func (s *Service) storeHasData() bool {
	unlock := s.store.rlock()
	defer unlock()
	return len(s.store.Systems) > 0
}

// seed 直接构造一份相互关联的演示状态。
func (s *Service) seed() error {
	unlock := s.store.lock()
	defer unlock()

	base := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	now := func(offsetDays int) time.Time { return base.AddDate(0, 0, offsetDays) }

	// 十个国家的制度背景。
	countries := []struct{ id, country, note string }{
		{"SYS-CN", "中国", "师范教育以院校课程方案为主，实习为集中教育见习与研习"},
		{"SYS-FI", "芬兰", "研究型硕士培养，教学实习嵌入大学附属师范校"},
		{"SYS-DE", "德国", "两阶段制：大学理论阶段与国家考试后的见习服务"},
		{"SYS-SG", "新加坡", "教育部与国立教育学院联合培养，CPD 与职称晋升强绑定"},
		{"SYS-US", "美国", "州级教师执照体系，学分与认证标准因州而异"},
		{"SYS-UK", "英国", "合格教师身份（QTS）框架，校本培训占比高"},
		{"SYS-JP", "日本", "教师执照课程法定化，初任者研修为强制入职辅导"},
		{"SYS-KR", "韩国", "教职课程与教职素养标准并行，在职研修学分化"},
		{"SYS-AU", "澳大利亚", "AITSL 教师专业标准统领初任/熟练/骨干三阶段"},
		{"SYS-NL", "荷兰", "大学与应用科技大学双轨，实习学时要求高"},
	}
	for _, c := range countries {
		s.store.Systems[c.id] = SystemContext{ID: c.id, Country: c.country, Description: c.note}
	}

	// 出处。
	s.store.Sources["SRC-1"] = Source{ID: "SRC-1", Citation: "OECD (2024) Education at a Glance, teacher initial preparation chapter", URL: "https://example.org/oecd-2024", RetrievedAt: now(-30)}
	s.store.Sources["SRC-2"] = Source{ID: "SRC-2", Citation: "示例师范院校 2026 本科培养方案（内部文件）", RetrievedAt: now(-20)}
	s.store.Sources["SRC-3"] = Source{ID: "SRC-3", Citation: "Finnish university teacher education curriculum 2024-2026", RetrievedAt: now(-18)}

	// 能力框架。
	s.store.Frameworks["FW-CN-2"] = Framework{
		ID: "FW-CN-2", SystemID: "SYS-CN", Name: "师范生核心素养框架（示例）", Version: "2",
		Domains: []FrameworkDomain{
			{Key: "practice", Name: "教育教学实践力"},
			{Key: "tech", Name: "数字素养与人工智能应用"},
			{Key: "ethics", Name: "师德与专业伦理"},
		},
	}
	s.store.Frameworks["FW-FI-1"] = Framework{
		ID: "FW-FI-1", SystemID: "SYS-FI", Name: "Finnish teacher competence map", Version: "1",
		Domains: []FrameworkDomain{
			{Key: "pedagogy", Name: "Research-based pedagogical expertise"},
			{Key: "digital", Name: "Digital and technological competence"},
		},
	}

	// 院校。
	s.store.Institutions["INST-CN-1"] = Institution{ID: "INST-CN-1", Name: "示例师范院校", Country: "中国", SystemID: "SYS-CN"}
	s.store.Institutions["INST-FI-1"] = Institution{ID: "INST-FI-1", Name: "示例芬兰大学教育学院", Country: "芬兰", SystemID: "SYS-FI"}

	// 课程版本：一个 AI 独立设课，一个融入式。
	s.store.Courses["C-CN-AI@2026"] = CourseVersion{
		ID: "C-CN-AI", Version: "2026", InstitutionID: "INST-CN-1", SystemID: "SYS-CN",
		Title: "人工智能教育基础", LocalTitle: "人工智能教育基础", Credits: 3, CreditUnit: "学分",
		AI:          AIDelivery{Mode: AIModeStandalone, Notes: "独立设课，含实验学时"},
		Practice:    PracticeComponent{Included: false},
		FrameworkID: "FW-CN-2", SourceIDs: []string{"SRC-2"},
		EffectiveFrom: "2026-09-01", RegisteredAt: now(-10),
	}
	s.store.Courses["C-FI-DIG@2025"] = CourseVersion{
		ID: "C-FI-DIG", Version: "2025", InstitutionID: "INST-FI-1", SystemID: "SYS-FI",
		Title: "Digital Competence in Teaching", Credits: 5, CreditUnit: "ECTS",
		AI:          AIDelivery{Mode: AIModeIntegrated, IntegratedInto: []string{"EDU-METHOD-2025"}, ContactHoursShare: 0.2, Notes: "AI 伦理与工具嵌入数字能力课"},
		Practice:    PracticeComponent{Included: true, Weeks: 4, Hours: 60, Mentored: true, Description: "附属师范校驻校实习"},
		FrameworkID: "FW-FI-1", SourceIDs: []string{"SRC-3"},
		EffectiveFrom: "2025-08-01", RegisteredAt: now(-12),
	}

	// CPD：日本强制初任者研修（演示"持续专业发展不是职前课程"）。
	s.store.CPDs["CPD-JP-IND"] = CPDProgram{
		ID: "CPD-JP-IND", SystemID: "SYS-JP", Name: "初任者研修", Stage: CPDInduction,
		Required: true, Hours: 120, SourceIDs: []string{"SRC-1"},
	}

	// 映射：M-1 数字素养 v1 已获批并支撑过政策建议；v2 为修订版（旧版被替代）。
	approvedAt := now(-5)
	proposedAt := now(-6)
	s.store.Mappings["M-1@1"] = &Mapping{
		ID: "M-1@1", LogicalID: "M-1", Version: 1, Kind: KindCompetency,
		Source:     MappingRef{SystemID: "SYS-CN", Kind: RefFrameworkDomain, RefID: "FW-CN-2#tech", Label: "数字素养与人工智能应用"},
		Target:     MappingRef{SystemID: "SYS-FI", Kind: RefFrameworkDomain, RefID: "FW-FI-1#digital", Label: "Digital and technological competence"},
		Confidence: 0.72, Applicability: "仅限职前阶段数字素养的总体要求比较；AI 专项深度不可比",
		Rationale: "两框架均要求面向教学的数字工具应用与伦理意识，但中方含独立 AI 课要求",
		ExpertID:  "expert-wang", Status: MappingSuperseded,
		Reviews:   []ReviewRecord{{ReviewerID: "reviewer-board", Decision: MappingApproved, At: approvedAt}},
		CreatedAt: proposedAt, DecidedAt: &approvedAt,
	}
	supersededAt := now(-1)
	supersededTime := supersededAt
	s.store.Mappings["M-1@2"] = &Mapping{
		ID: "M-1@2", LogicalID: "M-1", Version: 2, Supersedes: "M-1@1", Kind: KindCompetency,
		Source:     MappingRef{SystemID: "SYS-CN", Kind: RefFrameworkDomain, RefID: "FW-CN-2#tech", Label: "数字素养与人工智能应用"},
		Target:     MappingRef{SystemID: "SYS-FI", Kind: RefFrameworkDomain, RefID: "FW-FI-1#digital", Label: "Digital and technological competence"},
		Confidence: 0.6, Applicability: "仅限职前总体数字素养；明确排除 AI 独立课时比较",
		Rationale: "补充材料显示芬方 AI 内容仅占数字课约两成，置信度下调并收窄适用范围",
		ExpertID:  "expert-wang", Status: MappingApproved,
		Reviews:   []ReviewRecord{{ReviewerID: "reviewer-board", Decision: MappingApproved, At: supersededTime}},
		CreatedAt: supersededAt, DecidedAt: &supersededTime,
	}

	// M-2：AI 开设方式的课程级映射，仍待审——演示"不会产生虚假多数意见"。
	s.store.Mappings["M-2@1"] = &Mapping{
		ID: "M-2@1", LogicalID: "M-2", Version: 1, Kind: KindCourse,
		Source:     MappingRef{SystemID: "SYS-CN", Kind: RefCourse, RefID: "C-CN-AI@2026", Label: "人工智能教育基础（独立设课）"},
		Target:     MappingRef{SystemID: "SYS-FI", Kind: RefCourse, RefID: "C-FI-DIG@2025", Label: "Digital Competence in Teaching（融入式）"},
		Confidence: 0.4, Applicability: "仅用于课程定位参照，不能据此统计 AI 独立设课比例",
		Rationale: "独立设课与融入式的学分口径不同，需更多国家材料",
		ExpertID:  "expert-li", Status: MappingProposed, CreatedAt: now(-2),
	}

	// 政策建议：创建时冻结 M-1@1 快照（即便其现已被替代，仍可回溯）。
	snapM1v1 := *s.store.Mappings["M-1@1"]
	snapM1v1.Status = MappingApproved // 冻结的是获批时刻的状态，而非其现在的状态
	s.store.Recs["REC-1"] = PolicyRecommendation{
		ID: "REC-1", Title: "不建议以课程名称为依据统计各国 AI 独立设课比例",
		Basis: EvidenceBasis{
			SystemIDs:       []string{"SYS-CN", "SYS-FI"},
			MappingSnapshot: []Mapping{snapM1v1},
			CourseRefs:      []string{},
			PilotID:         "PILOT-1",
			CalibrationID:   "CAL-1@1",
			Notes:           "依据 M-1@1 获批时刻的内容",
		},
		CreatedAt: now(-4),
	}

	// 口径 v1 与试点。
	cal1 := &Calibration{
		ID: "CAL-1@1", Version: 1,
		CurriculumRefs: map[string]string{"INST-CN-1": "C-CN-AI@2026"},
		FrameworkRefs:  map[string]string{"INST-CN-1": "FW-CN-2"},
		Metrics: []MetricDefinition{
			{Key: "lesson_design", Name: "AI 融合教案评分", Definition: "按统一量规对每份教案 0-100 双盲评分", Unit: "分", HigherBetter: true},
			{Key: "retention", Name: "教师留任意向", Definition: "学期末匿名问卷 5 点量表均值", Unit: "量表均值", HigherBetter: true},
		},
		ExpectedInstitutions: []string{"INST-CN-1"},
		ExpectedCohorts:      []string{"2027届物理师范生", "2027届语文师范生"},
		MinResponseRate:      0.8,
		CreatedAt:            now(-3),
	}
	s.store.Calibrations[cal1.ID] = cal1

	concludedAt := now(0)
	pilot := &Pilot{
		ID: "PILOT-1", Name: "AI 融入教学法改革试点", PlanName: "2026 秋季试点方案",
		CalibrationID: "CAL-1@1", CalibrationVersion: 1,
		ParticipatingInstitutions: []string{"INST-CN-1"},
		TeacherCohorts:            []string{"2027届物理师范生", "2027届语文师范生"},
		Indicators:                []string{"lesson_design", "retention"},
		Stages: map[string]*PilotStageResult{
			"phase-1": {
				Stage: "phase-1", Period: "2026-09~2027-01",
				PresentIndicators: []string{"lesson_design", "retention"},
				Observations:      map[string]float64{"lesson_design": 82.5, "retention": 4.1},
				ResponseRates:     map[string]float64{"INST-CN-1": 0.91},
				ReportedAt:        now(-1),
				Conclusion:        "continue：两个群体样本完整、指标均达成预设方向，建议扩大到第二所院校",
				ConclusionAuthor:  "decision-group", ConcludedAt: &concludedAt, ConclusionCalibrationVer: 1,
			},
		},
		CreatedAt: now(-3),
	}
	s.store.Pilots["PILOT-1"] = pilot

	// 课程调整。
	s.store.Adjustments["ADJ-1"] = CourseAdjustment{
		ID: "ADJ-1", Title: "将 AI 内容由独立设课改为独立课+学科教学法融入",
		Description:       "依据中芬数字素养映射与首期试点结果提出，需在新口径下复测",
		RecommendationIDs: []string{"REC-1"}, PilotID: "PILOT-1", CreatedAt: now(0),
	}

	return s.store.persist()
}
