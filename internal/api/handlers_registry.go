package api

import (
	"net/http"

	"example.com/teacher-curriculum-evidence/internal/evidence"
)

func (s *Server) createSystem(w http.ResponseWriter, r *http.Request) {
	var in evidence.SystemContext
	if err := decode(r, &in); err != nil {
		writeError(w, badJSON(err))
		return
	}
	if err := s.svc.AddSystem(in); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) createInstitution(w http.ResponseWriter, r *http.Request) {
	var in evidence.Institution
	if err := decode(r, &in); err != nil {
		writeError(w, badJSON(err))
		return
	}
	if err := s.svc.AddInstitution(in); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var in evidence.Source
	if err := decode(r, &in); err != nil {
		writeError(w, badJSON(err))
		return
	}
	if err := s.svc.AddSource(in); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

type frameworkRequest struct {
	ID       string `json:"id"`
	SystemID string `json:"system_id"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Domains  []struct {
		Key   string `json:"key"`
		Name  string `json:"name"`
		Notes string `json:"notes"`
	} `json:"domains"`
}

func (s *Server) createFramework(w http.ResponseWriter, r *http.Request) {
	var req frameworkRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	fw := evidence.Framework{ID: req.ID, SystemID: req.SystemID, Name: req.Name, Version: req.Version}
	for _, d := range req.Domains {
		fw.Domains = append(fw.Domains, evidence.FrameworkDomain{Key: d.Key, Name: d.Name, Notes: d.Notes})
	}
	if err := s.svc.AddFramework(fw); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, fw)
}

type courseRequest struct {
	ID            string   `json:"id"`
	Version       string   `json:"version"`
	InstitutionID string   `json:"institution_id"`
	Title         string   `json:"title"`
	LocalTitle    string   `json:"local_title"`
	Credits       float64  `json:"credits"`
	CreditUnit    string   `json:"credit_unit"`
	Prerequisites []string `json:"prerequisites"`
	AI            struct {
		Mode              string   `json:"mode"`
		IntegratedInto    []string `json:"integrated_into"`
		ContactHoursShare float64  `json:"contact_hours_share"`
		Notes             string   `json:"notes"`
	} `json:"ai_delivery"`
	Practice struct {
		Included    bool   `json:"included"`
		Weeks       int    `json:"weeks"`
		Hours       int    `json:"hours"`
		Mentored    bool   `json:"mentored"`
		Description string `json:"description"`
	} `json:"practice"`
	FrameworkID   string   `json:"framework_id"`
	SourceIDs     []string `json:"source_ids"`
	EffectiveFrom string   `json:"effective_from"`
}

func (s *Server) createCourse(w http.ResponseWriter, r *http.Request) {
	var req courseRequest
	if err := decode(r, &req); err != nil {
		writeError(w, badJSON(err))
		return
	}
	in := evidence.AddCourseInput{
		ID:            req.ID,
		Version:       req.Version,
		InstitutionID: req.InstitutionID,
		Title:         req.Title,
		LocalTitle:    req.LocalTitle,
		Credits:       req.Credits,
		CreditUnit:    req.CreditUnit,
		Prerequisites: req.Prerequisites,
		AI: evidence.AIDelivery{
			Mode:              req.AI.Mode,
			IntegratedInto:    req.AI.IntegratedInto,
			ContactHoursShare: req.AI.ContactHoursShare,
			Notes:             req.AI.Notes,
		},
		Practice: evidence.PracticeComponent{
			Included:    req.Practice.Included,
			Weeks:       req.Practice.Weeks,
			Hours:       req.Practice.Hours,
			Mentored:    req.Practice.Mentored,
			Description: req.Practice.Description,
		},
		FrameworkID:   req.FrameworkID,
		SourceIDs:     req.SourceIDs,
		EffectiveFrom: req.EffectiveFrom,
	}
	cv, err := s.svc.AddCourse(in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cv)
}

func (s *Server) createCPD(w http.ResponseWriter, r *http.Request) {
	var in evidence.CPDProgram
	if err := decode(r, &in); err != nil {
		writeError(w, badJSON(err))
		return
	}
	if err := s.svc.AddCPD(in); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) listCourses(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.ListCourses())
}

func (s *Server) getCourse(w http.ResponseWriter, r *http.Request) {
	cv, err := s.svc.GetCourse(r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cv)
}
