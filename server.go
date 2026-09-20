package curriculum

import (
	"encoding/json"
	"errors"
	"net/http"
)

// NewHandler 装配证据服务的全部 HTTP 路由。
func NewHandler(svc *Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"状态": "服务已启动"})
	})

	mux.HandleFunc("POST /v1/countries", func(w http.ResponseWriter, r *http.Request) {
		var in Country
		if !decode(w, r, &in) {
			return
		}
		if err := svc.RegisterCountry(in); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, in)
	})

	mux.HandleFunc("POST /v1/institutions", func(w http.ResponseWriter, r *http.Request) {
		var in Institution
		if !decode(w, r, &in) {
			return
		}
		if err := svc.RegisterInstitution(in); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, in)
	})

	mux.HandleFunc("POST /v1/frameworks", func(w http.ResponseWriter, r *http.Request) {
		var in CompetencyFramework
		if !decode(w, r, &in) {
			return
		}
		if err := svc.RecordFramework(in); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, in)
	})

	mux.HandleFunc("POST /v1/courses", func(w http.ResponseWriter, r *http.Request) {
		var in Course
		if !decode(w, r, &in) {
			return
		}
		if err := svc.RecordCourse(in); err != nil {
			writeError(w, err)
			return
		}
		// 服务端补登记录时间后回读。
		saved, _ := svc.GetCourse(in.ID)
		writeJSON(w, http.StatusCreated, saved)
	})

	mux.HandleFunc("GET /v1/courses/{id}", func(w http.ResponseWriter, r *http.Request) {
		c, err := svc.GetCourse(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	})

	mux.HandleFunc("POST /v1/mappings", func(w http.ResponseWriter, r *http.Request) {
		var in MappingInput
		if !decode(w, r, &in) {
			return
		}
		m, err := svc.SubmitMapping(in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, m)
	})

	mux.HandleFunc("GET /v1/mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		m, err := svc.GetMapping(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("POST /v1/mappings/{id}/reviews", func(w http.ResponseWriter, r *http.Request) {
		var in MappingReview
		if !decode(w, r, &in) {
			return
		}
		m, err := svc.ReviewMapping(r.PathValue("id"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("POST /v1/mappings/{id}/objections", func(w http.ResponseWriter, r *http.Request) {
		var in Objection
		if !decode(w, r, &in) {
			return
		}
		m, err := svc.AddObjection(r.PathValue("id"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("POST /v1/mappings/{id}/revisions", func(w http.ResponseWriter, r *http.Request) {
		var in MappingInput
		if !decode(w, r, &in) {
			return
		}
		m, err := svc.ReviseMapping(r.PathValue("id"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, m)
	})

	mux.HandleFunc("GET /v1/comparisons/{concept}", func(w http.ResponseWriter, r *http.Request) {
		cmp, err := svc.CompareConcept(r.PathValue("concept"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, cmp)
	})

	mux.HandleFunc("POST /v1/calibers", func(w http.ResponseWriter, r *http.Request) {
		var in CaliberInput
		if !decode(w, r, &in) {
			return
		}
		cv, err := svc.CreateCaliber(in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, cv)
	})

	mux.HandleFunc("GET /v1/calibers/{id}", func(w http.ResponseWriter, r *http.Request) {
		cv, err := svc.GetCaliber(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, cv)
	})

	mux.HandleFunc("POST /v1/pilots", func(w http.ResponseWriter, r *http.Request) {
		var in PilotInput
		if !decode(w, r, &in) {
			return
		}
		p, err := svc.CreatePilot(in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, p)
	})

	mux.HandleFunc("GET /v1/pilots/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := svc.GetPilot(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("POST /v1/pilots/{id}/stages", func(w http.ResponseWriter, r *http.Request) {
		var in StageInput
		if !decode(w, r, &in) {
			return
		}
		st, err := svc.AddStage(r.PathValue("id"), in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, st)
	})

	mux.HandleFunc("POST /v1/recommendations", func(w http.ResponseWriter, r *http.Request) {
		var in RecommendationInput
		if !decode(w, r, &in) {
			return
		}
		rec, err := svc.AddRecommendation(in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, rec)
	})

	mux.HandleFunc("POST /v1/proposals", func(w http.ResponseWriter, r *http.Request) {
		var in ProposalInput
		if !decode(w, r, &in) {
			return
		}
		pp, err := svc.AddProposal(in)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, pp)
	})

	mux.HandleFunc("GET /v1/proposals/{id}/evidence-chain", func(w http.ResponseWriter, r *http.Request) {
		chain, err := svc.EvidenceChainFor(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, chain)
	})

	return mux
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"错误": "请求体不是合法 JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, ErrState):
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, map[string]string{"错误": err.Error()})
}
