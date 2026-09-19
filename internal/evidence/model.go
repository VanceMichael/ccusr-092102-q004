// Package evidence 保管教师教育课程改革的原始证据材料、跨体系映射、
// 政策建议谱系与改革试点的口径绑定结果。
//
// 核心约束：
//   - 各制度下的原始课程版本、能力框架、实践环节与持续专业发展分别登记，
//     不以课程名称作为可比性依据；
//   - 跨体系映射必须携带置信度、适用范围与异议，只有获批映射才进入正式比较；
//   - 政策建议留存获批映射的快照，映射修订后仍可回溯旧版曾支撑过哪些建议；
//   - 试点的方案、院校、教师群体、观察指标与阶段结果绑定同一口径版本，
//     样本缺失或统计定义改变时不得生成成效结论。
package evidence

import "time"

// SystemContext 描述一个国家/地区教师教育的制度背景。
type SystemContext struct {
	ID          string `json:"id"`
	Country     string `json:"country"`
	Description string `json:"description"`
	Notes       string `json:"notes,omitempty"`
}

// Institution 隶属某一制度背景的高校。
type Institution struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	SystemID string `json:"system_id"`
}

// Framework 是某制度下的教师核心素养/能力框架，按版本登记。
type Framework struct {
	ID       string `json:"id"`
	SystemID string `json:"system_id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	// Domains 保留该制度自己的素养领域命名，不预先翻译或对齐。
	Domains []FrameworkDomain `json:"domains"`
}

// FrameworkDomain 能力框架中的单个素养领域。
type FrameworkDomain struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Notes string `json:"notes,omitempty"`
}

// AI 内容的开设方式：独立设课、融入现有课程或未覆盖。
const (
	AIModeStandalone = "standalone" // 独立设课
	AIModeIntegrated = "integrated" // 融入现有课程
	AIModeNone       = "none"       // 未覆盖
)

// AIDelivery 描述 AI 相关内容在课程方案中的真实位置。
type AIDelivery struct {
	Mode              string   `json:"mode"`
	IntegratedInto    []string `json:"integrated_into,omitempty"`
	ContactHoursShare float64  `json:"contact_hours_share,omitempty"`
	Notes             string   `json:"notes,omitempty"`
}

// PracticeComponent 描述教育实习/临床实践环节。
type PracticeComponent struct {
	Included    bool   `json:"included"`
	Weeks       int    `json:"weeks,omitempty"`
	Hours       int    `json:"hours,omitempty"`
	Mentored    bool   `json:"mentored,omitempty"`
	Description string `json:"description,omitempty"`
}

// CourseVersion 是一所院校课程方案的一个不可变版本。
type CourseVersion struct {
	ID            string `json:"id"`
	Version       string `json:"version"`
	InstitutionID string `json:"institution_id"`
	SystemID      string `json:"system_id"`
	// Title/LocalTitle 保留原始名称与原文名称，名称不参与可比性判定。
	Title         string            `json:"title"`
	LocalTitle    string            `json:"local_title,omitempty"`
	Credits       float64           `json:"credits"`
	CreditUnit    string            `json:"credit_unit"`
	Prerequisites []string          `json:"prerequisites,omitempty"` // 形如 "C-1@2025"
	AI            AIDelivery        `json:"ai_delivery"`
	Practice      PracticeComponent `json:"practice"`
	FrameworkID   string            `json:"framework_id"`
	SourceIDs     []string          `json:"source_ids"`
	EffectiveFrom string            `json:"effective_from"`
	RegisteredAt  time.Time         `json:"registered_at"`
}

// CourseRef 返回课程版本的全局引用 "ID@Version"。
func (c CourseVersion) Ref() string { return c.ID + "@" + c.Version }

// CPDStage 持续专业发展阶段。
const (
	CPDStagePreservice = "preservice" // 职前
	CPDInduction       = "induction"  // 入职辅导
	CPDInService       = "inservice"  // 在职
)

// CPDProgram 描述制度性的持续专业发展安排，避免与职前课程混为一谈。
type CPDProgram struct {
	ID            string   `json:"id"`
	SystemID      string   `json:"system_id"`
	InstitutionID string   `json:"institution_id,omitempty"`
	Name          string   `json:"name"`
	Stage         string   `json:"stage"`
	Required      bool     `json:"required"`
	Hours         int      `json:"hours"`
	SourceIDs     []string `json:"source_ids"`
}

// Source 是一条研究/文件出处。
type Source struct {
	ID          string    `json:"id"`
	Citation    string    `json:"citation"`
	URL         string    `json:"url,omitempty"`
	RetrievedAt time.Time `json:"retrieved_at"`
}

// 映射状态。
const (
	MappingProposed   = "proposed"   // 专家提交，待审
	MappingApproved   = "approved"   // 获批，可进入正式比较
	MappingRejected   = "rejected"   // 审批驳回
	MappingSuperseded = "superseded" // 被修订版替代
)

// MappingRef 指向某制度内部的一个被映射对象（素养领域、课程版本或 CPD 项目）。
type MappingRef struct {
	SystemID string `json:"system_id"`
	Kind     string `json:"kind"` // framework_domain | course | cpd
	RefID    string `json:"ref_id"`
	Label    string `json:"label"`
}

// Objection 记录其他专家对映射的异议。
type Objection struct {
	ExpertID  string    `json:"expert_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// ReviewRecord 记录审批结论。
type ReviewRecord struct {
	ReviewerID string    `json:"reviewer_id"`
	Decision   string    `json:"decision"`
	Comment    string    `json:"comment"`
	At         time.Time `json:"at"`
}

// Mapping 是一条跨体系映射的某一版本实例（实例 ID 形如 "M-1@1"）。
type Mapping struct {
	ID            string         `json:"id"`
	LogicalID     string         `json:"logical_id"`
	Version       int            `json:"version"`
	Supersedes    string         `json:"supersedes,omitempty"`
	Kind          string         `json:"kind"` // competency | course | practice | cpd
	Source        MappingRef     `json:"source"`
	Target        MappingRef     `json:"target"`
	Confidence    float64        `json:"confidence"`
	Applicability string         `json:"applicability"`
	Rationale     string         `json:"rationale"`
	ExpertID      string         `json:"expert_id"`
	Status        string         `json:"status"`
	Objections    []Objection    `json:"objections,omitempty"`
	Reviews       []ReviewRecord `json:"reviews,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	DecidedAt     *time.Time     `json:"decided_at,omitempty"`
}

// PolicyRecommendation 是一项政策建议，其证据基础在创建时被冻结。
type PolicyRecommendation struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Basis     EvidenceBasis `json:"basis"`
	CreatedAt time.Time     `json:"created_at"`
}

// EvidenceBasis 是政策建议创建时刻的证据快照。
type EvidenceBasis struct {
	SystemIDs       []string  `json:"system_ids"`
	MappingSnapshot []Mapping `json:"mapping_snapshot"`
	CourseRefs      []string  `json:"course_refs"`
	PilotID         string    `json:"pilot_id,omitempty"`
	CalibrationID   string    `json:"calibration_id,omitempty"`
	Notes           string    `json:"notes,omitempty"`
}

// MetricDefinition 是观察指标在某一口径下的统计定义。
type MetricDefinition struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	Definition   string   `json:"definition"`
	Unit         string   `json:"unit"`
	Target       *float64 `json:"target,omitempty"`
	HigherBetter bool     `json:"higher_better"`
}

// Calibration 是试点绑定的统计口径版本。同一逻辑 ID 重新登记即生成新版本。
type Calibration struct {
	ID             string             `json:"id"`
	Version        int                `json:"version"`
	SupersededBy   string             `json:"superseded_by,omitempty"`
	CurriculumRefs map[string]string  `json:"curriculum_refs"` // 院校/课程 -> 课程版本引用
	FrameworkRefs  map[string]string  `json:"framework_refs"`
	Metrics        []MetricDefinition `json:"metrics"`
	// 样本范围要求
	ExpectedInstitutions []string  `json:"expected_institutions"`
	ExpectedCohorts      []string  `json:"expected_cohorts"`
	MinResponseRate      float64   `json:"min_response_rate"`
	CreatedAt            time.Time `json:"created_at"`
}

// PilotStageResult 是试点某一阶段上报的观察数据。
type PilotStageResult struct {
	Stage                    string             `json:"stage"`
	Period                   string             `json:"period"`
	PresentIndicators        []string           `json:"present_indicators"`
	Observations             map[string]float64 `json:"observations"`
	MissingInstitutions      []string           `json:"missing_institutions,omitempty"`
	MissingCohorts           []string           `json:"missing_cohorts,omitempty"`
	ResponseRates            map[string]float64 `json:"response_rates,omitempty"`
	ReportedAt               time.Time          `json:"reported_at"`
	Conclusion               string             `json:"conclusion,omitempty"`
	ConclusionAuthor         string             `json:"conclusion_author,omitempty"`
	ConcludedAt              *time.Time         `json:"concluded_at,omitempty"`
	ConclusionCalibrationVer int                `json:"conclusion_calibration_version,omitempty"`
}

// Pilot 是改革试点：方案、院校、教师群体、指标与阶段结果共用同一口径版本。
type Pilot struct {
	ID                        string                       `json:"id"`
	Name                      string                       `json:"name"`
	PlanName                  string                       `json:"plan_name"`
	CalibrationID             string                       `json:"calibration_id"`
	CalibrationVersion        int                          `json:"calibration_version"`
	ParticipatingInstitutions []string                     `json:"participating_institutions"`
	TeacherCohorts            []string                     `json:"teacher_cohorts"`
	Indicators                []string                     `json:"indicators"`
	Stages                    map[string]*PilotStageResult `json:"stages,omitempty"`
	CreatedAt                 time.Time                    `json:"created_at"`
}

// CourseAdjustment 是决策者提出的一次课程调整，通过证据链回溯依据。
type CourseAdjustment struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	RecommendationIDs []string  `json:"recommendation_ids"`
	PilotID           string    `json:"pilot_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
