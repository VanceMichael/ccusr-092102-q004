package evidence

import "testing"

// 快照持久化：落盘后重新打开，课程、映射状态与政策建议快照必须完整恢复。
func TestSnapshotPersistenceRoundTrip(t *testing.T) {
	svc := setup(t)
	path := svc.store.path

	id, err := svc.SubmitMapping(competencyMapping(0.7, "职前总体要求"))
	must(t, err)
	must(t, svc.ReviewMapping(id, "board", MappingApproved, "通过"))
	_, err = svc.CreateRecommendation(CreateRecommendationInput{
		ID: "REC-1", Title: "可对齐", MappingIDs: []string{id},
	})
	must(t, err)

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("重新打开快照失败: %v", err)
	}
	svc2 := NewService(reopened)

	cv, err := svc2.GetCourse("C-CN-AI@2026")
	must(t, err)
	if cv.Credits != 3 || cv.AI.Mode != AIModeStandalone {
		t.Fatalf("课程版本恢复内容不符: %+v", cv)
	}
	m, err := svc2.GetMapping("M-1@1")
	must(t, err)
	if m.Status != MappingApproved {
		t.Fatalf("映射审批状态应持久化，实际 %s", m.Status)
	}
	rec, err := svc2.GetRecommendation("REC-1")
	must(t, err)
	if len(rec.Basis.MappingSnapshot) != 1 || rec.Basis.MappingSnapshot[0].ID != "M-1@1" {
		t.Fatalf("政策建议证据快照应完整恢复，实际 %+v", rec.Basis)
	}
	backed := svc2.BackedRecommendations("M-1@1")
	if len(backed) != 1 {
		t.Fatalf("重开后仍应能从旧映射回溯建议，实际 %d", len(backed))
	}
}
