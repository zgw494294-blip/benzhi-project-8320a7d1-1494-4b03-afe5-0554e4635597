package httpapi

import (
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Server struct {
	app *application.Service
	mux *http.ServeMux
}

func New(app *application.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return requestID(JSONOnly(s.mux)) }
func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", Health)
	s.mux.HandleFunc("/api/v1/cores", s.cores)
	s.mux.HandleFunc("/api/v1/cores/", s.coreResource)
	s.mux.HandleFunc("/api/v1/cases", s.cases)
	s.mux.HandleFunc("/api/v1/cases/", s.caseResource)
	s.mux.HandleFunc("/api/v1/available/", s.available)
	s.mux.HandleFunc("/api/v1/credentials/", s.credential)
}
func write(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	status := http.StatusBadRequest
	code := "INVALID_REQUEST"
	message := e.Error()
	var details any
	var be *domain.BusinessError
	if errors.As(e, &be) {
		code = be.Code
		message = be.Message
		details = be.Details
	}
	if errors.Is(e, domain.ErrNotFound) {
		status = 404
		if code == "INVALID_REQUEST" {
			code = "NOT_FOUND"
		}
	} else if errors.Is(e, domain.ErrConflict) {
		status = 409
		if code == "INVALID_REQUEST" {
			code = "CONFLICT"
		}
	} else if errors.Is(e, domain.ErrState) {
		status = 422
		if code == "INVALID_REQUEST" {
			code = "INVALID_STATE"
		}
	}
	write(w, map[string]any{"error": map[string]any{"code": code, "message": message, "details": details}}, status)
}
func decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return domain.NewError(domain.ErrInvalid, "INVALID_JSON", "请求正文只能包含一个 JSON 值", nil)
	}
	return nil
}
func method(w http.ResponseWriter, r *http.Request, want string) bool {
	if r.Method != want {
		w.Header().Set("Allow", want)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func (s *Server) cores(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		write(w, s.app.ListCores(), 200)
		return
	}
	if !method(w, r, http.MethodPost) {
		return
	}
	var c domain.CoreRecord
	if e := decode(r, &c); e != nil {
		fail(w, e)
		return
	}
	if e := s.app.RegisterCore(c); e != nil {
		fail(w, e)
		return
	}
	v, _ := s.app.GetCore(c.CoreID)
	write(w, v, 201)
}
func (s *Server) coreResource(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		w.WriteHeader(404)
		return
	}
	id, action := parts[3], parts[4]
	switch action {
	case "impact", "impact-preview", "revise":
		if !method(w, r, http.MethodPost) {
			return
		}
		var q struct {
			ExpectedRevision     int               `json:"expectedRevision"`
			BoxID                string            `json:"boxId"`
			MinimumReserveMassMg int               `json:"minimumReserveMassMg"`
			ProtectedIntervals   []domain.Interval `json:"protectedIntervals"`
			RevisionNote         string            `json:"revisionNote"`
		}
		if e := decode(r, &q); e != nil {
			fail(w, e)
			return
		}
		cmd := application.CoreRevisionCommand{ExpectedRevision: q.ExpectedRevision, BoxID: q.BoxID, MinimumReserveMassMg: q.MinimumReserveMassMg, ProtectedIntervals: q.ProtectedIntervals, RevisionNote: q.RevisionNote}
		if action == "impact" || action == "impact-preview" {
			v, e := s.app.PreviewCoreRevision(id, cmd)
			if e != nil {
				fail(w, e)
				return
			}
			write(w, v, 200)
			return
		}
		v, impact, e := s.app.ReviseCoreDetailed(id, cmd)
		if e != nil {
			fail(w, e)
			return
		}
		affected := make([]string, 0, len(impact.AffectedCases))
		for _, item := range impact.AffectedCases {
			affected = append(affected, item.CaseID)
		}
		write(w, map[string]any{"core": v.Core, "revision": v.Core.Revision, "changeSummary": v.ChangeSummary, "affectedCaseIds": affected}, 200)
	case "revisions", "versions":
		if !method(w, r, http.MethodGet) {
			return
		}
		v, e := s.app.CoreVersions(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	default:
		w.WriteHeader(404)
	}
}
func (s *Server) cases(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var q struct {
		domain.SamplingCase
		ExpectedRevision int `json:"expectedRevision"`
	}
	if e := decode(r, &q); e != nil {
		fail(w, e)
		return
	}
	v, e := s.app.CreateCase(q.SamplingCase, q.ExpectedRevision)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, v, 201)
}
func (s *Server) caseResource(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		w.WriteHeader(404)
		return
	}
	id := parts[3]
	if len(parts) == 4 {
		if !method(w, r, http.MethodGet) {
			return
		}
		v, e := s.app.GetCase(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
		return
	}
	action := parts[4]
	switch action {
	case "revisions":
		if !method(w, r, http.MethodGet) {
			return
		}
		v, e := s.app.CaseVersions(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	case "revise":
		if !method(w, r, http.MethodPost) {
			return
		}
		var q struct {
			ExpectedRevision  int              `json:"expectedRevision"`
			Purpose           string           `json:"purpose"`
			Method            string           `json:"method"`
			RequestedSegments []domain.Segment `json:"requestedSegments"`
			EstimatedMassMg   int              `json:"estimatedMassMg"`
			RevisionNote      string           `json:"revisionNote"`
			Actor             string           `json:"actor"`
		}
		if e := decode(r, &q); e != nil {
			fail(w, e)
			return
		}
		v, e := s.app.ReviseCaseCommand(id, application.CaseRevisionCommand{ExpectedRevision: q.ExpectedRevision, Purpose: q.Purpose, Method: q.Method, RequestedSegments: q.RequestedSegments, EstimatedMassMg: q.EstimatedMassMg, RevisionNote: q.RevisionNote, Actor: q.Actor})
		if e != nil {
			fail(w, e)
			return
		}
		write(w, map[string]any{"case": v.Case, "revision": v.Case.Revision, "changeSummary": v.ChangeSummary, "recheckRequired": true}, 200)
	case "precheck":
		if len(parts) >= 6 {
			s.precheckResource(w, r, id, parts)
			return
		}
		if !method(w, r, http.MethodPost) {
			return
		}
		var q struct{}
		if e := decode(r, &q); e != nil && e != io.EOF {
			fail(w, e)
			return
		}
		v, e := s.app.Precheck(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	case "prechecks":
		s.precheckResource(w, r, id, parts)
	case "submit":
		if !method(w, r, http.MethodPost) {
			return
		}
		var q struct{}
		if e := decode(r, &q); e != nil && e != io.EOF {
			fail(w, e)
			return
		}
		v, e := s.app.SubmitCase(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	case "authorize":
		if !method(w, r, http.MethodPost) {
			return
		}
		var q struct{}
		if e := decode(r, &q); e != nil && e != io.EOF {
			fail(w, e)
			return
		}
		v, e := s.app.Authorize(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	case "authorization":
		if !method(w, r, http.MethodGet) {
			return
		}
		v, e := s.app.Authorization(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	case "review":
		s.review(w, r, id)
	case "findings":
		s.findings(w, r, id, parts)
	case "execute":
		s.execute(w, r, id)
	case "receipts", "execution-receipts", "executions":
		if len(parts) != 6 || !method(w, r, http.MethodGet) {
			if len(parts) != 6 {
				w.WriteHeader(404)
			}
			return
		}
		v, e := s.app.ExecutionReceipt(id, parts[5])
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
	case "freeze":
		s.freeze(w, r, id)
	case "verifications", "verification-attempts":
		s.verifications(w, r, id, parts)
	default:
		w.WriteHeader(404)
	}
}
func (s *Server) precheckResource(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	if len(parts) < 6 || !method(w, r, http.MethodGet) {
		if len(parts) < 6 {
			w.WriteHeader(404)
		}
		return
	}
	p, v, e := s.app.GetPrecheck(id, parts[5])
	if e != nil {
		fail(w, e)
		return
	}
	write(w, map[string]any{"snapshot": p, "validity": v}, 200)
}
func (s *Server) review(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var q struct {
		Findings []struct {
			Code       string          `json:"code"`
			Message    string          `json:"message"`
			SegmentRef *domain.Segment `json:"segmentRef"`
		} `json:"findings"`
		Code       string          `json:"code"`
		Message    string          `json:"message"`
		SegmentRef *domain.Segment `json:"segmentRef"`
	}
	if e := decode(r, &q); e != nil {
		fail(w, e)
		return
	}
	issues := make([]application.ReviewIssue, 0, len(q.Findings))
	for _, x := range q.Findings {
		issues = append(issues, application.ReviewIssue{Code: x.Code, Message: x.Message, SegmentRef: x.SegmentRef})
	}
	if len(issues) == 0 && (q.Code != "" || q.Message != "") {
		issues = append(issues, application.ReviewIssue{Code: q.Code, Message: q.Message, SegmentRef: q.SegmentRef})
	}
	v, e := s.app.ReturnCaseIssues(id, issues)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, v, 200)
}
func (s *Server) findings(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	if len(parts) == 5 {
		if !method(w, r, http.MethodGet) {
			return
		}
		rev, _ := strconv.Atoi(r.URL.Query().Get("revision"))
		items, events, e := s.app.Findings(id, r.URL.Query().Get("status"), rev)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, map[string]any{"findings": items, "events": events}, 200)
		return
	}
	if len(parts) != 7 || parts[6] != "close" || !method(w, r, http.MethodPost) {
		w.WriteHeader(404)
		return
	}
	var q struct {
		ClosureNote  string `json:"closureNote"`
		Note         string `json:"note"`
		CaseRevision int    `json:"caseRevision"`
	}
	if e := decode(r, &q); e != nil {
		fail(w, e)
		return
	}
	if q.ClosureNote == "" {
		q.ClosureNote = q.Note
	}
	v, e := s.app.CloseFindingWithRevision(id, parts[5], q.ClosureNote, q.CaseRevision)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, v, 200)
}
func (s *Server) execute(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var q struct {
		IdempotencyKey      string           `json:"idempotencyKey"`
		AuthorizationDigest string           `json:"authorizationDigest"`
		ActualSegments      []domain.Segment `json:"actualSegments"`
		MassBeforeMg        int              `json:"massBeforeMg"`
		SampleMassMg        int              `json:"sampleMassMg"`
		MassAfterMg         int              `json:"massAfterMg"`
		ContainerCode       string           `json:"containerCode"`
		Operator            string           `json:"operator"`
		Witness             string           `json:"witness"`
	}
	if e := decode(r, &q); e != nil {
		fail(w, e)
		return
	}
	v, e := s.app.ExecuteReceiptContext(r.Context(), id, q.IdempotencyKey, q.AuthorizationDigest, q.ActualSegments, q.MassBeforeMg, q.SampleMassMg, q.MassAfterMg, q.ContainerCode, q.Operator, q.Witness)
	if e != nil {
		fail(w, e)
		return
	}
	status := 201
	if v.Replayed {
		status = 200
	}
	write(w, v, status)
}
func (s *Server) freeze(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var q struct {
		RemainingMassMg int    `json:"remainingMassMg"`
		StorageLocation string `json:"storageLocation"`
		Verifier        string `json:"verifier"`
		WitnessNote     string `json:"witnessNote"`
		VerificationKey string `json:"verificationKey"`
	}
	if e := decode(r, &q); e != nil {
		fail(w, e)
		return
	}
	v, e := s.app.VerifyAndFreeze(id, application.FreezeCommand{RemainingMassMg: q.RemainingMassMg, StorageLocation: q.StorageLocation, Verifier: q.Verifier, WitnessNote: q.WitnessNote, VerificationKey: q.VerificationKey})
	if e != nil {
		fail(w, e)
		return
	}
	status := 200
	if v.Verdict == "Passed" {
		status = 201
	}
	write(w, v, status)
}
func (s *Server) verifications(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	if !method(w, r, http.MethodGet) {
		return
	}
	if len(parts) == 5 {
		v, e := s.app.VerificationAttempts(id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
		return
	}
	if len(parts) == 6 {
		v, e := s.app.VerificationAttempt(id, parts[5])
		if e != nil {
			fail(w, e)
			return
		}
		write(w, v, 200)
		return
	}
	w.WriteHeader(404)
}
func (s *Server) available(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/available/")
	v, e := s.app.Available(id)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, v, 200)
}
func (s *Server) credential(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/credentials/")
	v, e := s.app.VerifyCredential(id)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, v, 200)
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		next.ServeHTTP(w, r)
	})
}
