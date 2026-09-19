package evidence

import (
	"fmt"
	"sort"
)

// ProposeAdjustmentInput 提出一次课程调整。
type ProposeAdjustmentInput struct {
	ID                string
	Title             string
	Description       string
	RecommendationIDs []string
	PilotID           string
}

// ProposeAdjustment 登记决策者提出的课程调整，关联政策建议与（可选的）试点。
func (s *Service) ProposeAdjustment(in ProposeAdjustmentInput) (CourseAdjustment, error) {
	if in.ID == "" || in.Title == "" {
		return CourseAdjustment{}, fmt.Errorf("%w: 调整 id 与标题不能为空", ErrInvalid)
	}
	unlock := s.store.lock()
	defer unlock()
	if _, exists := s.store.Adjustments[in.ID]; exists {
		return CourseAdjustment{}, fmt.Errorf("%w: 调整 %s", ErrAlreadyExists, in.ID)
	}
	for _, rid := range in.RecommendationIDs {
		if _, ok := s.store.Recs[rid]; !ok {
			return CourseAdjustment{}, fmt.Errorf("%w: 政策建议 %s", ErrNotFound, rid)
		}
	}
	if in.PilotID != "" {
		if _, ok := s.store.Pilots[in.PilotID]; !ok {
			return CourseAdjustment{}, fmt.Errorf("%w: 试点 %s", ErrNotFound, in.PilotID)
		}
	}
	adj := CourseAdjustment{
		ID:                in.ID,
		Title:             in.Title,
		Description:       in.Description,
		RecommendationIDs: append([]string(nil), in.RecommendationIDs...),
		PilotID:           in.PilotID,
		CreatedAt:         s.now(),
	}
	s.store.Adjustments[in.ID] = adj
	return adj, s.store.persist()
}

// ComparableMaterial 描述证据链中"真正可比"的一份制度材料。
type ComparableMaterial struct {
	MappingID      string   `json:"mapping_id"`
	Kind           string   `json:"kind"`
	SourceSystem   string   `json:"source_system"`
	TargetSystem   string   `json:"target_system"`
	Confidence     float64  `json:"confidence"`
	Applicability  string   `json:"applicability"`
	Disputed       bool     `json:"disputed"`
	Rationale      string   `json:"rationale"`
	BackedByRecIDs []string `json:"backed_by_recommendations"`
}

// Gap 描述证据链中尚无共识的部分。
type Gap struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	// 相关的映射实例（待审/驳回/被替代），如有。
	MappingID string `json:"mapping_id,omitempty"`
	SystemID  string `json:"system_id,omitempty"`
}

// PilotStageEvidence 试点阶段数据为何支撑当前结论。
type PilotStageEvidence struct {
	Stage              string             `json:"stage"`
	Period             string             `json:"period"`
	CalibrationID      string             `json:"calibration_id"`
	CalibrationVersion int                `json:"calibration_version"`
	Fingerprint        string             `json:"fingerprint"`
	Indicators         []string           `json:"indicators"`
	Observations       map[string]float64 `json:"observations"`
	GatePassed         bool               `json:"gate_passed"`
	Conclusion         string             `json:"conclusion,omitempty"`
	ConclusionAuthor   string             `json:"conclusion_author,omitempty"`
}

// RecommendationEvidence 政策建议及其快照新旧状况。
type RecommendationEvidence struct {
	Recommendation PolicyRecommendation    `json:"recommendation"`
	Freshness      RecommendationFreshness `json:"freshness"`
}

// EvidenceChain 是一次课程调整可沿之回溯的完整证据链。
type EvidenceChain struct {
	AdjustmentID        string                   `json:"adjustment_id"`
	ComparableMaterials []ComparableMaterial     `json:"comparable_materials"`
	Gaps                []Gap                    `json:"gaps"`
	Recommendations     []RecommendationEvidence `json:"recommendations"`
	Pilot               *Pilot                   `json:"pilot,omitempty"`
	PilotStages         []PilotStageEvidence     `json:"pilot_stages,omitempty"`
	// AutoConclusionBlocked 为 true 表示存在未通过闸门、因此不得自动生成结论的阶段。
	StagesWithoutData []string `json:"stages_without_data,omitempty"`
	GeneratedAt       string   `json:"generated_at"`
}

// BuildEvidenceChain 沿课程调整聚合证据：哪些国家材料真正可比、哪些部分尚无共识、
// 试点数据为何支持继续/改动/停止。
func (s *Service) BuildEvidenceChain(adjustmentID string) (EvidenceChain, error) {
	unlock := s.store.rlock()
	defer unlock()
	adj, ok := s.store.Adjustments[adjustmentID]
	if !ok {
		return EvidenceChain{}, fmt.Errorf("%w: 调整 %s", ErrNotFound, adjustmentID)
	}
	chain := EvidenceChain{
		AdjustmentID:        adjustmentID,
		ComparableMaterials: []ComparableMaterial{},
		Gaps:                []Gap{},
		Recommendations:     []RecommendationEvidence{},
		GeneratedAt:         s.now().Format("2006-01-02T15:04:05Z07:00"),
	}

	// 1. 政策建议及其映射快照、新旧状况。
	approvedSet := map[string]bool{}
	for _, rid := range adj.RecommendationIDs {
		rec := s.store.Recs[rid]
		fresh := RecommendationFreshness{RecommendationID: rid, AllCurrent: true, Entries: []SnapshotFreshness{}}
		for _, snap := range rec.Basis.MappingSnapshot {
			latest, status := 0, ""
			for _, m := range s.store.Mappings {
				if m.LogicalID == snap.LogicalID && m.Version > latest {
					latest = m.Version
					status = m.Status
				}
			}
			entry := SnapshotFreshness{
				MappingID: snap.ID, CurrentStatus: status, LatestVersion: latest,
				SnapshotVersion: snap.Version,
				Changed:         latest != snap.Version || status != snap.Status,
			}
			if entry.Changed {
				fresh.AllCurrent = false
			}
			fresh.Entries = append(fresh.Entries, entry)
			if snap.Status == MappingApproved {
				approvedSet[snap.ID] = true
			}
		}
		chain.Recommendations = append(chain.Recommendations, RecommendationEvidence{Recommendation: rec, Freshness: fresh})
	}

	// 2. 真正可比的材料：建议快照中且当前仍获批未替代的映射。
	// 已被修订/替代或撤销批准的快照进入 Gaps，而不是可比清单。
	listed := map[string]bool{}
	for _, rid := range adj.RecommendationIDs {
		rec := s.store.Recs[rid]
		for _, snap := range rec.Basis.MappingSnapshot {
			current, exists := s.store.Mappings[snap.ID]
			if exists && current.Status == MappingApproved {
				if !listed[snap.ID] {
					chain.ComparableMaterials = append(chain.ComparableMaterials, ComparableMaterial{
						MappingID:      snap.ID,
						Kind:           snap.Kind,
						SourceSystem:   snap.Source.SystemID,
						TargetSystem:   snap.Target.SystemID,
						Confidence:     snap.Confidence,
						Applicability:  snap.Applicability,
						Disputed:       len(snap.Objections) > 0,
						Rationale:      snap.Rationale,
						BackedByRecIDs: s.recsBackingMapping(snap.ID),
					})
					listed[snap.ID] = true
				}
			} else {
				status := "已被删除"
				if exists {
					status = current.Status
				}
				desc := fmt.Sprintf("曾支撑建议 %s 的映射 %s 当前状态为 %s，不再具备正式可比性", rid, snap.ID, status)
				if latest, ok := s.latestMapping(snap.LogicalID); ok && latest.ID != snap.ID {
					desc += fmt.Sprintf("；同概念最新版本为 %s（状态 %s），如需沿用须以其重新立据", latest.ID, latest.Status)
				}
				chain.Gaps = append(chain.Gaps, Gap{
					Kind:        snap.Kind,
					Description: desc,
					MappingID:   snap.ID,
				})
			}
		}
	}
	sort.Slice(chain.ComparableMaterials, func(i, j int) bool {
		return chain.ComparableMaterials[i].MappingID < chain.ComparableMaterials[j].MappingID
	})

	// 3. 尚无共识：所有待审/驳回/被替代的映射（与本调整相关类别），
	//    以及未被任何获批映射覆盖的制度。
	relevantKinds := map[string]bool{}
	for _, cm := range chain.ComparableMaterials {
		relevantKinds[cm.Kind] = true
	}
	for _, rec := range chain.Recommendations {
		for _, snap := range rec.Recommendation.Basis.MappingSnapshot {
			relevantKinds[snap.Kind] = true
		}
	}
	covered := map[string]bool{}
	for _, m := range s.store.Mappings {
		if m.Status == MappingApproved {
			covered[m.Source.SystemID] = true
			covered[m.Target.SystemID] = true
		}
	}
	// 已在建议快照中处理过的映射不再重复列入缺口。
	alreadyGapped := map[string]bool{}
	for _, g := range chain.Gaps {
		alreadyGapped[g.MappingID] = true
	}
	for _, m := range s.store.Mappings {
		if !relevantKinds[m.Kind] {
			continue
		}
		if alreadyGapped[m.ID] {
			continue
		}
		switch m.Status {
		case MappingProposed:
			chain.Gaps = append(chain.Gaps, Gap{
				Kind: m.Kind, Description: fmt.Sprintf("映射 %s 尚待审批，不进入正式比较", m.ID), MappingID: m.ID,
			})
		case MappingRejected:
			chain.Gaps = append(chain.Gaps, Gap{
				Kind: m.Kind, Description: fmt.Sprintf("映射 %s 已被驳回：%s", m.ID, lastObjection(m)), MappingID: m.ID,
			})
		case MappingSuperseded:
			desc := fmt.Sprintf("映射 %s 已被修订版替代", m.ID)
			if latest, ok := s.latestMapping(m.LogicalID); ok && latest.Version > m.Version {
				switch latest.Status {
				case MappingApproved:
					desc += fmt.Sprintf("，最新版本 %s 已获批，正式比较以新版为准", latest.ID)
				case MappingProposed:
					desc += fmt.Sprintf("，最新版本 %s 尚待审批，该概念暂时无共识", latest.ID)
				case MappingRejected:
					desc += fmt.Sprintf("，最新版本 %s 已被驳回，该概念暂时无共识", latest.ID)
				default:
					desc += fmt.Sprintf("，最新版本为 %s", latest.ID)
				}
			}
			chain.Gaps = append(chain.Gaps, Gap{Kind: m.Kind, Description: desc, MappingID: m.ID})
		}
	}
	for sysID := range s.store.Systems {
		if !covered[sysID] {
			chain.Gaps = append(chain.Gaps, Gap{
				Kind: "system", Description: fmt.Sprintf("制度 %s 尚无任何获批跨体系映射，该方向不具备可比性", sysID), SystemID: sysID,
			})
		}
	}
	sort.Slice(chain.Gaps, func(i, j int) bool {
		if chain.Gaps[i].MappingID != chain.Gaps[j].MappingID {
			return chain.Gaps[i].MappingID < chain.Gaps[j].MappingID
		}
		return chain.Gaps[i].SystemID < chain.Gaps[j].SystemID
	})

	// 4. 试点：阶段闸门状态与结论。
	if adj.PilotID != "" {
		pilot := s.store.Pilots[adj.PilotID]
		chain.Pilot = pilot
		cal := s.store.Calibrations[pilot.CalibrationID]
		for stageName, res := range pilot.Stages {
			chain.PilotStages = append(chain.PilotStages, PilotStageEvidence{
				Stage:              stageName,
				Period:             res.Period,
				CalibrationID:      pilot.CalibrationID,
				CalibrationVersion: pilot.CalibrationVersion,
				Fingerprint:        cal.Fingerprint(),
				Indicators:         res.PresentIndicators,
				Observations:       res.Observations,
				GatePassed:         true, // 能写入即代表三道闸门通过
				Conclusion:         res.Conclusion,
				ConclusionAuthor:   res.ConclusionAuthor,
			})
		}
		sort.Slice(chain.PilotStages, func(i, j int) bool { return chain.PilotStages[i].Stage < chain.PilotStages[j].Stage })
		// 口径已变则显式提示禁止沿用旧口径结论。
		if cal.SupersededBy != "" {
			chain.Gaps = append(chain.Gaps, Gap{
				Kind:        "calibration",
				Description: fmt.Sprintf("试点口径 %s 已被 %s 替代，历史阶段结论仅可按其当时口径解读，不得外推", cal.ID, cal.SupersededBy),
			})
		}
	}

	return chain, nil
}

func (s *Service) recsBackingMapping(mappingID string) []string {
	var out []string
	for _, rec := range s.store.Recs {
		for _, m := range rec.Basis.MappingSnapshot {
			if m.ID == mappingID {
				out = append(out, rec.ID)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *Service) latestMapping(logicalID string) (*Mapping, bool) {
	var latest *Mapping
	for _, m := range s.store.Mappings {
		if m.LogicalID == logicalID && (latest == nil || m.Version > latest.Version) {
			latest = m
		}
	}
	return latest, latest != nil
}

func lastObjection(m *Mapping) string {
	if len(m.Objections) == 0 {
		return "无书面理由"
	}
	return m.Objections[len(m.Objections)-1].Reason
}
