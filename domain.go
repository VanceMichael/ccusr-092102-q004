// Package curriculum 保管教师教育课程的原始证据，并对跨体系映射、
// 口径版本、改革试点与政策证据链实施统一的业务规则。
package curriculum

import "errors"

// 业务规则相关的哨兵错误，HTTP 层与调用方据此判定状态码。
var (
	// ErrInvalid 输入不合法。
	ErrInvalid = errors.New("输入不合法")
	// ErrNotFound 资源不存在。
	ErrNotFound = errors.New("资源不存在")
	// ErrConflict 与既有不可变记录冲突。
	ErrConflict = errors.New("记录冲突")
	// ErrState 当前状态不允许该操作（如映射未审批、试点存在阻断项）。
	ErrState = errors.New("当前状态不允许该操作")
)

// 制度背景：一个国家/地区教师教育体系的关键约束。
// 跨体系比较必须连同这些背景一起阅读，不能只看课程名称。
type Country struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	SystemContext  string `json:"system_context"`  // 学位结构、资格框架、准入与监管制度
	EducationStage string `json:"education_stage"` // 该登记覆盖的学段
}

// Institution 参与证据登记的院校。
type Institution struct {
	ID          string `json:"id"`
	CountryCode string `json:"country_code"`
	Name        string `json:"name"`
}

// SourceReference 研究出处：原始文件、官方目录或学术来源。
type SourceReference struct {
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	PublishedOn string `json:"published_on,omitempty"`
	Kind        string `json:"kind"` // official_catalogue | regulation | research | handbook
}

// PracticeComponent 实践环节（实习、驻校、微格教学等），按原始方案记录。
type PracticeComponent struct {
	Type          string `json:"type"` // practicum | school_placement | microteaching
	DurationWeeks int    `json:"duration_weeks"`
	Required      bool   `json:"required"`
	Description   string `json:"description,omitempty"`
}

// DeliveryMode 某主题在原始方案中的开设方式。
type DeliveryMode string

const (
	// DeliveryStandalone 独立设课。
	DeliveryStandalone DeliveryMode = "standalone"
	// DeliveryEmbedded 融入现有课程。
	DeliveryEmbedded DeliveryMode = "embedded"
)

// Course 是某院校某版培养方案中的原始课程记录。一经登记不可修改，
// 方案修订须以新的 program_version 重新登记，以保留历史版本。
type Course struct {
	ID             string              `json:"id"`
	InstitutionID  string              `json:"institution_id"`
	ProgramVersion string              `json:"program_version"` // 原始培养方案版本
	LocalCode      string              `json:"local_code"`      // 院校原始课程代码
	NameLocal      string              `json:"name_local"`      // 原文名称
	NameTranslated string              `json:"name_translated,omitempty"`
	Credits        float64             `json:"credits"`
	CreditSystem   string              `json:"credit_system"` // ECTS | credits | hours 等原始学分制
	Prerequisites  []string            `json:"prerequisites"` // 先修课程原始代码
	IsCPD          bool                `json:"is_cpd"`        // 是否为职后持续专业发展课程
	Practice       []PracticeComponent `json:"practice,omitempty"`
	// TopicTags 标注课程覆盖的规范主题（如 ai-in-education），
	// 由登记员依据出处标注，供跨体系检索；不等于体系间已达成映射共识。
	TopicTags []string `json:"topic_tags,omitempty"`
	// DeliveryByTopic 记录每个主题是独立设课还是融入课程，必须有出处支撑。
	DeliveryByTopic map[string]DeliveryMode `json:"delivery_by_topic,omitempty"`
	CompetencyRefs  []string                `json:"competency_refs,omitempty"` // 本地能力框架条目代码
	Sources         []SourceReference       `json:"sources"`
	EffectiveFrom   string                  `json:"effective_from,omitempty"`
	RecordedOn      string                  `json:"recorded_on"`
}

// CompetencyFramework 某体系/院校的能力框架版本。
type CompetencyFramework struct {
	ID            string           `json:"id"`
	InstitutionID string           `json:"institution_id,omitempty"` // 空表示国家层面框架
	CountryCode   string           `json:"country_code"`
	Version       string           `json:"version"`
	Name          string           `json:"name"`
	Items         []CompetencyItem `json:"items"`
	Source        SourceReference  `json:"source"`
	EffectiveFrom string           `json:"effective_from,omitempty"`
}

// CompetencyItem 能力框架中的单条素养，保留原始定义原文。
type CompetencyItem struct {
	Code            string `json:"code"`
	Description     string `json:"description"`
	DescriptionOrig string `json:"description_orig,omitempty"` // 原始语言表述
}

// 映射生命周期状态。
const (
	MappingSubmitted  = "submitted"  // 已提交，待审批，不进入正式比较
	MappingApproved   = "approved"   // 已批准，可进入口径与正式比较
	MappingRejected   = "rejected"   // 未获批准
	MappingSuperseded = "superseded" // 已被修订版替代
)

// ConceptAnchor 某体系内与规范概念对应的本地锚点。
type ConceptAnchor struct {
	CountryCode string `json:"country_code"`
	// Kind 锚点类型：competency（能力条目）| course_topic（课程主题）|
	// practice（实践环节）| cpd（持续专业发展）。
	Kind string `json:"kind"`
	// Ref 本地引用，如 "FW2#C3"（框架#条目）、主题标签或实践类型代码。
	Ref string `json:"ref"`
}

// MappingScope 映射意见的适用范围。
type MappingScope struct {
	EducationStage string   `json:"education_stage,omitempty"` // 适用学段
	ProgramTypes   []string `json:"program_types,omitempty"`   // 适用的培养方案类型
	Notes          string   `json:"notes,omitempty"`
}

// Objection 专家对映射的异议；未解决的异议使该锚点在比较中标注为“有争议”。
type Objection struct {
	Expert     string `json:"expert"`
	On         string `json:"on"`
	Reason     string `json:"reason"`
	Resolution string `json:"resolution,omitempty"` // 空表示尚未解决
}

// Mapping 跨体系映射意见。每次修订生成新记录并保留修订链，
// 旧版曾支撑的政策建议因此仍可追溯。
type Mapping struct {
	ID           string        `json:"id"`
	RevisionOf   string        `json:"revision_of,omitempty"` // 上一版映射 ID
	Concept      string        `json:"concept"`               // 规范概念代码，如 ai-in-education、core-literacy、practicum、cpd
	Anchor       ConceptAnchor `json:"anchor"`
	Relationship string        `json:"relationship"` // equivalent | narrower | broader | related
	Expert       string        `json:"expert"`
	Confidence   float64       `json:"confidence"` // 0~1
	Basis        string        `json:"basis"`      // 判定依据
	Scope        MappingScope  `json:"scope"`
	Status       string        `json:"status"`
	Objections   []Objection   `json:"objections,omitempty"`
	SubmittedOn  string        `json:"submitted_on"`
	ReviewedBy   string        `json:"reviewed_by,omitempty"`
	ReviewedOn   string        `json:"reviewed_on,omitempty"`
	ReviewNote   string        `json:"review_note,omitempty"`
}

// HasUnresolvedObjection 报告是否存在尚未解决的异议。
func (m *Mapping) HasUnresolvedObjection() bool {
	for _, o := range m.Objections {
		if o.Resolution == "" {
			return true
		}
	}
	return false
}

// MetricDefinition 观察指标的统计口径定义。口径冻结后不可修改，
// 定义变化必须建立新的口径版本。
type MetricDefinition struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	Unit       string `json:"unit,omitempty"`
}

// CaliberVersion 口径版本：冻结一组已批准映射与一套指标定义，
// 试点与正式比较都必须指明使用的口径。
type CaliberVersion struct {
	ID                 string             `json:"id"`
	Label              string             `json:"label"`
	Notes              string             `json:"notes,omitempty"`
	ApprovedMappingIDs []string           `json:"approved_mapping_ids"`
	Metrics            []MetricDefinition `json:"metrics"`
	CreatedOn          string             `json:"created_on"`
}

// TeacherCohort 试点覆盖的教师群体。
type TeacherCohort struct {
	Label       string `json:"label"`
	Size        int    `json:"size"`
	Description string `json:"description,omitempty"`
}

// IndicatorResult 单个观察指标在某阶段的取值与样本情况。
type IndicatorResult struct {
	MetricCode            string   `json:"metric_code"`
	Value                 *float64 `json:"value,omitempty"`
	SampleSize            int      `json:"sample_size"`
	ExpectedSample        int      `json:"expected_sample"`
	MissingReason         string   `json:"missing_reason,omitempty"`
	DefinitionFingerprint string   `json:"definition_fingerprint,omitempty"` // 采数时依据的指标定义指纹
}

// PilotStage 试点的一个阶段及其结果。
type PilotStage struct {
	ID         string             `json:"id"`
	Label      string             `json:"label"`
	RecordedOn string             `json:"recorded_on"`
	Results    []IndicatorResult  `json:"results"`
	Blockers   []string           `json:"blockers,omitempty"` // 样本缺失/定义变更等阻断项
	Conclusion *OutcomeConclusion `json:"conclusion,omitempty"`
}

// OutcomeConclusion 对阶段数据的显式结论。只有不存在阻断项时才允许记录。
type OutcomeConclusion struct {
	RecordedBy string `json:"recorded_by"`
	On         string `json:"on"`
	Decision   string `json:"decision"` // continue | modify | stop
	Rationale  string `json:"rationale"`
}

// Pilot 改革试点：方案、院校、教师群体、指标与阶段结果全部绑定同一口径版本。
type Pilot struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	CaliberID      string          `json:"caliber_id"`
	Plan           string          `json:"plan"`
	InstitutionIDs []string        `json:"institution_ids"`
	Cohorts        []TeacherCohort `json:"cohorts"`
	Indicators     []string        `json:"indicators"` // 必须取自口径中的指标代码
	Stages         []PilotStage    `json:"stages,omitempty"`
	CreatedOn      string          `json:"created_on"`
}

// PolicyRecommendation 政策建议：精确引用支撑它的映射版本与试点阶段。
type PolicyRecommendation struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Detail     string   `json:"detail"`
	CaliberID  string   `json:"caliber_id"`
	PilotID    string   `json:"pilot_id"`
	StageIDs   []string `json:"stage_ids"`
	MappingIDs []string `json:"mapping_ids"` // 精确到映射版本
	CreatedOn  string   `json:"created_on"`
}

// PolicyProposal 决策者提出的一次课程调整，可据此回溯完整证据链。
type PolicyProposal struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	ProposedBy        string   `json:"proposed_by"`
	On                string   `json:"on"`
	RecommendationIDs []string `json:"recommendation_ids"`
}
