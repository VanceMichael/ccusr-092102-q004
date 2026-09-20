package curriculum_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	curriculum "example.com/teacher-curriculum-evidence"
)

func newTestServer(t *testing.T) (*curriculum.Service, http.Handler) {
	t.Helper()
	s := curriculum.NewService()
	return s, curriculum.NewHandler(s)
}

// fingerprint 取口径内指标定义指纹，供构造阶段结果时使用。
func fingerprint(t *testing.T, s *curriculum.Service, caliber, metric string) string {
	t.Helper()
	cv, err := s.GetCaliber(caliber)
	if err != nil {
		t.Fatal(err)
	}
	return cv.MetricFingerprint(metric)
}

func do(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != "" {
		rdr = bytes.NewReader([]byte(body))
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestHealth(t *testing.T) {
	_, h := newTestServer(t)
	code, body := do(t, h, "GET", "/health", "")
	if code != http.StatusOK {
		t.Fatalf("健康检查应为 200，得到 %d", code)
	}
	if body["状态"] != "服务已启动" {
		t.Fatalf("健康检查响应异常: %v", body)
	}
}

func TestHTTPEndToEndEvidenceFlow(t *testing.T) {
	s, h := newTestServer(t)

	// 制度背景与院校。
	for _, body := range []string{
		`{"code":"CN","name":"中国","system_context":"教师资格证制度","education_stage":"中学"}`,
		`{"code":"DE","name":"德国","system_context":"国家考试","education_stage":"文理中学"}`,
	} {
		code, _ := do(t, h, "POST", "/v1/countries", body)
		if code != http.StatusCreated {
			t.Fatalf("登记国家应 201，得到 %d", code)
		}
	}
	code, _ := do(t, h, "POST", "/v1/institutions",
		`{"id":"cn-u","country_code":"CN","name":"示范师大"}`)
	if code != http.StatusCreated {
		t.Fatalf("登记院校应 201，得到 %d", code)
	}
	code, _ = do(t, h, "POST", "/v1/institutions",
		`{"id":"de-u","country_code":"DE","name":"慕尼黑师大"}`)
	if code != http.StatusCreated {
		t.Fatalf("登记院校应 201，得到 %d", code)
	}

	// 原始课程：独立设课（CN）与融入（DE）。
	code, _ = do(t, h, "POST", "/v1/courses", `{
		"id":"cn-c1","institution_id":"cn-u","program_version":"2026-A",
		"local_code":"AI-01","name_local":"人工智能教育基础","credits":2,
		"credit_system":"credits","topic_tags":["ai-in-education"],
		"delivery_by_topic":{"ai-in-education":"standalone"},
		"sources":[{"title":"课程目录","kind":"official_catalogue"}]}`)
	if code != http.StatusCreated {
		t.Fatalf("登记课程应 201，得到 %d", code)
	}
	code, _ = do(t, h, "POST", "/v1/courses", `{
		"id":"de-c1","institution_id":"de-u","program_version":"2025WS",
		"local_code":"FD-210","name_local":"Fachdidaktik digital","credits":6,
		"credit_system":"ECTS","topic_tags":["ai-in-education"],
		"delivery_by_topic":{"ai-in-education":"embedded"},
		"sources":[{"title":"模块手册","kind":"handbook"}]}`)
	if code != http.StatusCreated {
		t.Fatalf("登记课程应 201，得到 %d", code)
	}

	// 提交两条映射，此时正式比较必须被拒绝（422）。
	for _, b := range []string{
		`{"concept":"ai-in-education","anchor":{"country_code":"CN","kind":"course_topic","ref":"ai-in-education"},"relationship":"equivalent","expert":"张","confidence":0.85,"basis":"课程目录"}`,
		`{"concept":"ai-in-education","anchor":{"country_code":"DE","kind":"course_topic","ref":"ai-in-education"},"relationship":"narrower","expert":"Müller","confidence":0.7,"basis":"模块手册"}`,
	} {
		code, _ = do(t, h, "POST", "/v1/mappings", b)
		if code != http.StatusCreated {
			t.Fatalf("提交映射应 201，得到 %d", code)
		}
	}
	code, body := do(t, h, "GET", "/v1/comparisons/ai-in-education", "")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("未审批时正式比较应 422，得到 %d", code)
	}
	if !strings.Contains(body["错误"].(string), "尚无已批准映射") {
		t.Fatalf("应说明无已批准映射，得到 %v", body)
	}

	// 未知字段应 400；审批两条映射。
	code, _ = do(t, h, "POST", "/v1/mappings/M001/reviews",
		`{"reviewer":"委员会","approve":true,"unexpected":1}`)
	if code != http.StatusBadRequest {
		t.Fatalf("未知字段应 400，得到 %d", code)
	}
	for _, id := range []string{"M001", "M002"} {
		code, _ = do(t, h, "POST", "/v1/mappings/"+id+"/reviews",
			`{"reviewer":"委员会","approve":true}`)
		if code != http.StatusOK {
			t.Fatalf("审批 %s 应 200，得到 %d", id, code)
		}
	}

	// 正式比较：两国都有原始出处，投票各 1，且不产生“多数国家独立设课”的假象。
	code, body = do(t, h, "GET", "/v1/comparisons/ai-in-education", "")
	if code != http.StatusOK {
		t.Fatalf("正式比较应 200，得到 %d", code)
	}
	vote, _ := body["delivery_vote"].(map[string]any)
	if vote["standalone"].(float64) != 1 || vote["embedded"].(float64) != 1 {
		t.Fatalf("开设方式投票应各为 1，得到 %v", vote)
	}

	// 口径冻结 + 试点。
	code, _ = do(t, h, "POST", "/v1/calibers", `{
		"id":"cal-1","label":"第一口径","approved_mapping_ids":["M001","M002"],
		"metrics":[{"code":"m_pass","name":"通过率","definition":"合格/参加","unit":"比例"}]}`)
	if code != http.StatusCreated {
		t.Fatalf("创建口径应 201，得到 %d", code)
	}
	code, _ = do(t, h, "POST", "/v1/pilots", `{
		"id":"p1","name":"试点","caliber_id":"cal-1","plan":"工作坊方案",
		"institution_ids":["cn-u"],"cohorts":[{"label":"在职教师","size":40}],
		"indicators":["m_pass"]}`)
	if code != http.StatusCreated {
		t.Fatalf("创建试点应 201，得到 %d", code)
	}

	// 样本缺失却下结论 → 422；不带结论可入库 201。
	code, body = do(t, h, "POST", "/v1/pilots/p1/stages", `{
		"id":"s1","label":"中期","results":[
			{"metric_code":"m_pass","sample_size":20,"expected_sample":40,
			 "missing_reason":"两校未交回","definition_fingerprint":"`+fingerprint(t, s, "cal-1", "m_pass")+`"}],
		"conclusion":{"recorded_by":"组","decision":"continue","rationale":"x"}}`)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("样本缺失时下结论应 422，得到 %d（%v）", code, body)
	}
	code, _ = do(t, h, "POST", "/v1/pilots/p1/stages", `{
		"id":"s1","label":"中期","results":[
			{"metric_code":"m_pass","sample_size":20,"expected_sample":40,
			 "missing_reason":"两校未交回","definition_fingerprint":"`+fingerprint(t, s, "cal-1", "m_pass")+`"}]}`)
	if code != http.StatusCreated {
		t.Fatalf("无结论的阻断阶段应允许入库 201，得到 %d", code)
	}

	// 完整阶段 → 结论入库；再生成建议、提案与证据链。
	code, _ = do(t, h, "POST", "/v1/pilots/p1/stages", `{
		"id":"s2","label":"终期","results":[
			{"metric_code":"m_pass","sample_size":40,"expected_sample":40,"value":0.82,
			 "definition_fingerprint":"`+fingerprint(t, s, "cal-1", "m_pass")+`"}],
		"conclusion":{"recorded_by":"专家组","decision":"modify","rationale":"整体良好但需加强实践"}}`)
	if code != http.StatusCreated {
		t.Fatalf("完整阶段应 201，得到 %d", code)
	}
	code, _ = do(t, h, "POST", "/v1/recommendations", `{
		"id":"r1","title":"调整建议","detail":"加强实践环节","pilot_id":"p1",
		"stage_ids":["s2"],"mapping_ids":["M001"]}`)
	if code != http.StatusCreated {
		t.Fatalf("政策建议应 201，得到 %d", code)
	}
	code, _ = do(t, h, "POST", "/v1/proposals", `{
		"id":"pp1","title":"课程调整","description":"加强实践","proposed_by":"决策者",
		"recommendation_ids":["r1"]}`)
	if code != http.StatusCreated {
		t.Fatalf("提案应 201，得到 %d", code)
	}
	code, body = do(t, h, "GET", "/v1/proposals/pp1/evidence-chain", "")
	if code != http.StatusOK {
		t.Fatalf("证据链应 200，得到 %d", code)
	}
	decisions, _ := body["decisions"].([]any)
	if len(decisions) != 1 || decisions[0] != "s2: modify" {
		t.Fatalf("证据链应汇总显式决策 s2: modify，得到 %v", decisions)
	}
	blocked, _ := body["blocked_stages"].([]any)
	// s1 虽未被建议直接引用，但属于同一试点；其样本缺失阻断必须在证据链中可见。
	if len(blocked) != 1 {
		t.Fatalf("证据链应暴露 1 个阻断阶段，得到 %d", len(blocked))
	}
	b0, _ := blocked[0].(map[string]any)
	if b0["id"] != "s1" {
		t.Fatalf("阻断阶段应为 s1，得到 %v", b0["id"])
	}
}

func TestHTTPNotFound(t *testing.T) {
	_, h := newTestServer(t)
	code, _ := do(t, h, "GET", "/v1/courses/nope", "")
	if code != http.StatusNotFound {
		t.Fatalf("不存在资源应 404，得到 %d", code)
	}
}
