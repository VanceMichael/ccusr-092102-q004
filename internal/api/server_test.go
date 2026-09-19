package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"example.com/teacher-curriculum-evidence/internal/evidence"
)

func newTestServer(t *testing.T) (*Server, *evidence.Service) {
	t.Helper()
	st, err := evidence.NewStore(filepath.Join(t.TempDir(), "evidence.json"))
	if err != nil {
		t.Fatalf("存储初始化失败: %v", err)
	}
	svc := evidence.NewService(st)
	return NewServer(svc), svc
}

func do(t *testing.T, h http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func seedBasics(t *testing.T, svc *evidence.Service) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(svc.AddSystem(evidence.SystemContext{ID: "SYS-CN", Country: "中国"}))
	must(svc.AddSystem(evidence.SystemContext{ID: "SYS-FI", Country: "芬兰"}))
	must(svc.AddSource(evidence.Source{ID: "SRC-1", Citation: "示例出处"}))
	must(svc.AddFramework(evidence.Framework{
		ID: "FW-CN-1", SystemID: "SYS-CN", Name: "中方框架", Version: "1",
		Domains: []evidence.FrameworkDomain{{Key: "tech", Name: "数字素养"}},
	}))
	must(svc.AddFramework(evidence.Framework{
		ID: "FW-FI-1", SystemID: "SYS-FI", Name: "芬方框架", Version: "1",
		Domains: []evidence.FrameworkDomain{{Key: "digital", Name: "Digital competence"}},
	}))
	must(svc.AddInstitution(evidence.Institution{ID: "INST-CN", Name: "示例师大", SystemID: "SYS-CN"}))
	_, err := svc.AddCourse(evidence.AddCourseInput{
		ID: "C-CN-AI", Version: "2026", InstitutionID: "INST-CN", Title: "AI 教育基础",
		Credits: 3, AI: evidence.AIDelivery{Mode: evidence.AIModeStandalone},
		FrameworkID: "FW-CN-1", SourceIDs: []string{"SRC-1"},
	})
	must(err)
}

// 端到端：待审映射不进入比较；审批需走复核接口；样本缺失时阶段上报返回 422。
func TestHTTPEndToEndGates(t *testing.T) {
	srv, svc := newTestServer(t)
	seedBasics(t, svc)
	h := srv.Handler()

	// 提交映射后未审批：比较结果为空。
	status, _ := do(t, h, "POST", "/v1/mappings", map[string]any{
		"kind": "competency",
		"source": map[string]string{
			"system_id": "SYS-CN", "kind": "framework_domain", "ref_id": "FW-CN-1#tech",
		},
		"target": map[string]string{
			"system_id": "SYS-FI", "kind": "framework_domain", "ref_id": "FW-FI-1#digital",
		},
		"confidence": 0.7, "applicability": "仅限职前总体要求", "expert_id": "expert-wang",
	})
	if status != http.StatusCreated {
		t.Fatalf("提交映射应返回 201，实际 %d", status)
	}
	status, body := do(t, h, "GET", "/v1/comparisons/competency", nil)
	if status != http.StatusOK || len(body["comparable_pairs"].([]any)) != 0 {
		t.Fatalf("未审批映射不得进入比较，status=%d body=%v", status, body)
	}

	// 用未获批映射立政策建议：422。
	status, _ = do(t, h, "POST", "/v1/recommendations", map[string]any{
		"id": "REC-BAD", "title": "过早的建议", "mapping_ids": []string{"M-1@1"},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("未获批映射立据应返回 422，实际 %d", status)
	}

	// 驳回但不附理由：422。
	status, _ = do(t, h, "POST", "/v1/mappings/M-1@1/reviews", map[string]string{
		"reviewer_id": "board", "decision": "rejected", "comment": "",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("无理由驳回应返回 422，实际 %d", status)
	}

	// 注册口径并建试点，缺样本上报应被拒。
	status, _ = do(t, h, "POST", "/v1/calibrations", map[string]any{
		"logical_id":            "CAL-1",
		"curriculum_refs":       map[string]string{"INST-CN": "C-CN-AI@2026"},
		"metrics":               []map[string]any{{"key": "m1", "name": "指标1", "definition": "定义1", "unit": "分"}},
		"expected_institutions": []string{"INST-CN"},
		"expected_cohorts":      []string{"物理师范生"},
		"min_response_rate":     0.8,
	})
	if status != http.StatusCreated {
		t.Fatalf("登记口径应返回 201，实际 %d", status)
	}
	status, _ = do(t, h, "POST", "/v1/pilots", map[string]any{
		"id": "PILOT-1", "name": "试点", "plan_name": "方案甲", "calibration_id": "CAL-1@1",
		"participating_institutions": []string{"INST-CN"},
		"teacher_cohorts":            []string{"物理师范生"},
		"indicators":                 []string{"m1"},
	})
	if status != http.StatusCreated {
		t.Fatalf("建试点应返回 201，实际 %d", status)
	}
	status, body = do(t, h, "POST", "/v1/pilots/PILOT-1/stages", map[string]any{
		"stage": "phase-1", "present_indicators": []string{"m1"},
		"observations":         map[string]float64{"m1": 80},
		"missing_institutions": []string{"INST-CN"},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("样本缺失应返回 422，实际 %d：%v", status, body)
	}
}
