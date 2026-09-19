// Package api 提供证据服务的 HTTP 接口。
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"example.com/teacher-curriculum-evidence/internal/evidence"
)

// Server 装配路由与 JSON 处理。
type Server struct {
	svc *evidence.Service
}

// NewServer 创建 HTTP 服务。
func NewServer(svc *evidence.Service) *Server {
	return &Server{svc: svc}
}

// Handler 返回带全部路由的 http.Handler。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.health)

	mux.HandleFunc("POST /v1/systems", s.createSystem)
	mux.HandleFunc("POST /v1/institutions", s.createInstitution)
	mux.HandleFunc("POST /v1/sources", s.createSource)
	mux.HandleFunc("POST /v1/frameworks", s.createFramework)
	mux.HandleFunc("POST /v1/courses", s.createCourse)
	mux.HandleFunc("POST /v1/cpds", s.createCPD)
	mux.HandleFunc("GET /v1/courses", s.listCourses)
	mux.HandleFunc("GET /v1/courses/{ref}", s.getCourse)

	mux.HandleFunc("POST /v1/mappings", s.submitMapping)
	mux.HandleFunc("POST /v1/mappings/{id}/revise", s.reviseMapping)
	mux.HandleFunc("POST /v1/mappings/{id}/objections", s.addObjection)
	mux.HandleFunc("POST /v1/mappings/{id}/reviews", s.reviewMapping)
	mux.HandleFunc("GET /v1/mappings/{id}", s.getMapping)
	mux.HandleFunc("GET /v1/mapping-lineages/{logicalID}", s.mappingLineage)
	mux.HandleFunc("GET /v1/comparisons/{kind}", s.compare)

	mux.HandleFunc("POST /v1/recommendations", s.createRecommendation)
	mux.HandleFunc("GET /v1/recommendations/{id}", s.getRecommendation)
	mux.HandleFunc("GET /v1/recommendations/{id}/freshness", s.recommendationFreshness)
	mux.HandleFunc("GET /v1/backed-recommendations", s.backedRecommendations)

	mux.HandleFunc("POST /v1/calibrations", s.registerCalibration)
	mux.HandleFunc("POST /v1/pilots", s.createPilot)
	mux.HandleFunc("GET /v1/pilots/{id}", s.getPilot)
	mux.HandleFunc("POST /v1/pilots/{id}/stages", s.reportStage)
	mux.HandleFunc("POST /v1/pilots/{id}/stages/{stage}/conclusion", s.recordConclusion)

	mux.HandleFunc("POST /v1/adjustments", s.proposeAdjustment)
	mux.HandleFunc("GET /v1/adjustments/{id}/evidence-chain", s.evidenceChain)

	return loggingMiddleware(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"状态": "服务已启动"})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// ---- JSON 辅助 ----

func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, evidence.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, evidence.ErrAlreadyExists):
		status = http.StatusConflict
	case errors.Is(err, evidence.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, evidence.ErrMappingNotApproved),
		errors.Is(err, evidence.ErrMappingNotPending),
		errors.Is(err, evidence.ErrSystemMismatch),
		errors.Is(err, evidence.ErrMissingPrerequisite),
		errors.Is(err, evidence.ErrFrameworkMissing),
		errors.Is(err, evidence.ErrObjectionMissing):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, evidence.ErrCalibrationChanged),
		errors.Is(err, evidence.ErrCalibrationMissing),
		errors.Is(err, evidence.ErrSampleIncomplete),
		errors.Is(err, evidence.ErrDefinitionChanged),
		errors.Is(err, evidence.ErrIndicatorNotInScope),
		errors.Is(err, evidence.ErrPilotMismatch):
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
