package api

import (
	"fmt"
	"net/http"

	"example.com/teacher-curriculum-evidence/internal/evidence"
)

type recommendationRequest struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	MappingIDs []string `json:"mapping_ids"`
	PilotID    string   `json:"pilot_id"`
	Notes      string   `json:"notes"`
}

func (s *Server) createRecommendation(w http.ResponseWriter, r *http.Request) {
	var req recommendationRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	rec, err := s.svc.CreateRecommendation(evidence.CreateRecommendationInput{
		ID: req.ID, Title: req.Title, MappingIDs: req.MappingIDs, PilotID: req.PilotID, Notes: req.Notes,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) getRecommendation(w http.ResponseWriter, r *http.Request) {
	rec, err := s.svc.GetRecommendation(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) recommendationFreshness(w http.ResponseWriter, r *http.Request) {
	fresh, err := s.svc.CheckRecommendationFreshness(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fresh)
}

func (s *Server) backedRecommendations(w http.ResponseWriter, r *http.Request) {
	mappingID := r.URL.Query().Get("mapping_id")
	if mappingID == "" {
		writeError(w, fmt.Errorf("%w: 查询参数 mapping_id 不能为空", evidence.ErrInvalid))
		return
	}
	writeJSON(w, http.StatusOK, s.svc.BackedRecommendations(mappingID))
}

type calibrationRequest struct {
	LogicalID            string                      `json:"logical_id"`
	CurriculumRefs       map[string]string           `json:"curriculum_refs"`
	FrameworkRefs        map[string]string           `json:"framework_refs"`
	Metrics              []evidence.MetricDefinition `json:"metrics"`
	ExpectedInstitutions []string                    `json:"expected_institutions"`
	ExpectedCohorts      []string                    `json:"expected_cohorts"`
	MinResponseRate      float64                     `json:"min_response_rate"`
}

func (s *Server) registerCalibration(w http.ResponseWriter, r *http.Request) {
	var req calibrationRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	cal, err := s.svc.RegisterCalibration(evidence.RegisterCalibrationInput{
		LogicalID: req.LogicalID, CurriculumRefs: req.CurriculumRefs, FrameworkRefs: req.FrameworkRefs,
		Metrics: req.Metrics, ExpectedInstitutions: req.ExpectedInstitutions,
		ExpectedCohorts: req.ExpectedCohorts, MinResponseRate: req.MinResponseRate,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cal)
}

type pilotRequest struct {
	ID                        string   `json:"id"`
	Name                      string   `json:"name"`
	PlanName                  string   `json:"plan_name"`
	CalibrationID             string   `json:"calibration_id"`
	ParticipatingInstitutions []string `json:"participating_institutions"`
	TeacherCohorts            []string `json:"teacher_cohorts"`
	Indicators                []string `json:"indicators"`
}

func (s *Server) createPilot(w http.ResponseWriter, r *http.Request) {
	var req pilotRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	pilot, err := s.svc.CreatePilot(evidence.CreatePilotInput{
		ID: req.ID, Name: req.Name, PlanName: req.PlanName, CalibrationID: req.CalibrationID,
		ParticipatingInstitutions: req.ParticipatingInstitutions,
		TeacherCohorts:            req.TeacherCohorts,
		Indicators:                req.Indicators,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pilot)
}

func (s *Server) getPilot(w http.ResponseWriter, r *http.Request) {
	pilot, err := s.svc.GetPilot(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pilot)
}

type stageRequest struct {
	Stage               string             `json:"stage"`
	Period              string             `json:"period"`
	PresentIndicators   []string           `json:"present_indicators"`
	Observations        map[string]float64 `json:"observations"`
	MissingInstitutions []string           `json:"missing_institutions"`
	MissingCohorts      []string           `json:"missing_cohorts"`
	ResponseRates       map[string]float64 `json:"response_rates"`
}

func (s *Server) reportStage(w http.ResponseWriter, r *http.Request) {
	var req stageRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	res, err := s.svc.ReportStage(evidence.ReportStageInput{
		PilotID: r.PathValue("id"), Stage: req.Stage, Period: req.Period,
		PresentIndicators: req.PresentIndicators, Observations: req.Observations,
		MissingInstitutions: req.MissingInstitutions, MissingCohorts: req.MissingCohorts,
		ResponseRates: req.ResponseRates,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type conclusionRequest struct {
	Decision string `json:"decision"`
	Author   string `json:"author"`
	Comment  string `json:"comment"`
}

func (s *Server) recordConclusion(w http.ResponseWriter, r *http.Request) {
	var req conclusionRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	pilot, err := s.svc.RecordConclusion(evidence.RecordConclusionInput{
		PilotID: r.PathValue("id"), Stage: r.PathValue("stage"),
		Decision: req.Decision, Author: req.Author, Comment: req.Comment,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pilot)
}

type adjustmentRequest struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	RecommendationIDs []string `json:"recommendation_ids"`
	PilotID           string   `json:"pilot_id"`
}

func (s *Server) proposeAdjustment(w http.ResponseWriter, r *http.Request) {
	var req adjustmentRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	adj, err := s.svc.ProposeAdjustment(evidence.ProposeAdjustmentInput{
		ID: req.ID, Title: req.Title, Description: req.Description,
		RecommendationIDs: req.RecommendationIDs, PilotID: req.PilotID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, adj)
}

func (s *Server) evidenceChain(w http.ResponseWriter, r *http.Request) {
	chain, err := s.svc.BuildEvidenceChain(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chain)
}
