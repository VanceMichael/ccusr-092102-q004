package api

import (
	"fmt"
	"net/http"

	"example.com/teacher-curriculum-evidence/internal/evidence"
)

// badJSON 把解码错误包装成参数不合法，供 writeError 映射 400。
func badJSON(err error) error {
	return fmt.Errorf("%w: 请求体不是合法 JSON（%v）", evidence.ErrInvalid, err)
}

type mappingRequest struct {
	LogicalID     string              `json:"logical_id"`
	Kind          string              `json:"kind"`
	Source        evidence.MappingRef `json:"source"`
	Target        evidence.MappingRef `json:"target"`
	Confidence    float64             `json:"confidence"`
	Applicability string              `json:"applicability"`
	Rationale     string              `json:"rationale"`
	ExpertID      string              `json:"expert_id"`
}

func (s *Server) submitMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	id, err := s.svc.SubmitMapping(evidence.SubmitMappingInput{
		LogicalID: req.LogicalID, Kind: req.Kind, Source: req.Source, Target: req.Target,
		Confidence: req.Confidence, Applicability: req.Applicability, Rationale: req.Rationale, ExpertID: req.ExpertID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	m, _ := s.svc.GetMapping(id)
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) reviseMapping(w http.ResponseWriter, r *http.Request) {
	var req mappingRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	id, err := s.svc.ReviseMapping(r.PathValue("id"), req.ExpertID, evidence.SubmitMappingInput{
		Kind: req.Kind, Source: req.Source, Target: req.Target,
		Confidence: req.Confidence, Applicability: req.Applicability, Rationale: req.Rationale, ExpertID: req.ExpertID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	m, _ := s.svc.GetMapping(id)
	writeJSON(w, http.StatusCreated, m)
}

type objectionRequest struct {
	ExpertID string `json:"expert_id"`
	Reason   string `json:"reason"`
}

func (s *Server) addObjection(w http.ResponseWriter, r *http.Request) {
	var req objectionRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	if err := s.svc.AddObjection(r.PathValue("id"), req.ExpertID, req.Reason); err != nil {
		writeError(w, err)
		return
	}
	m, err := s.svc.GetMapping(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

type reviewRequest struct {
	ReviewerID string `json:"reviewer_id"`
	Decision   string `json:"decision"`
	Comment    string `json:"comment"`
}

func (s *Server) reviewMapping(w http.ResponseWriter, r *http.Request) {
	var req reviewRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	if err := s.svc.ReviewMapping(r.PathValue("id"), req.ReviewerID, req.Decision, req.Comment); err != nil {
		writeError(w, err)
		return
	}
	m, err := s.svc.GetMapping(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) getMapping(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.GetMapping(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) mappingLineage(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.MappingLineage(r.PathValue("logicalID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) compare(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.Compare(r.PathValue("kind"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
