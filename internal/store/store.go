package store

import (
	"corepreservation/internal/domain"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type snapshot struct {
	Cores                map[string]domain.CoreRecord            `json:"cores"`
	CoreVersions         map[string][]domain.CoreVersion         `json:"coreVersions"`
	Cases                map[string]domain.SamplingCase          `json:"cases"`
	CaseVersions         map[string][]domain.CaseVersion         `json:"caseVersions"`
	Prechecks            map[string]domain.PrecheckSnapshot      `json:"prechecks"`
	Authorizations       map[string]domain.AuthorizationManifest `json:"authorizations"`
	Executions           map[string]domain.CutExecution          `json:"executions"`
	ExecutionReceipts    map[string]domain.ExecutionReceipt      `json:"executionReceipts"`
	VerificationAttempts map[string][]domain.VerificationAttempt `json:"verificationAttempts"`
	FindingEvents        map[string][]domain.FindingEvent        `json:"findingEvents"`
	Credentials          map[string]domain.ProvenanceCredential  `json:"credentials"`
}
type Store struct {
	mu               sync.RWMutex
	dir              string
	data             snapshot
	verificationKeys map[string]domain.VerificationAttempt
}

func emptySnapshot() snapshot {
	return snapshot{Cores: map[string]domain.CoreRecord{}, CoreVersions: map[string][]domain.CoreVersion{}, Cases: map[string]domain.SamplingCase{}, CaseVersions: map[string][]domain.CaseVersion{}, Prechecks: map[string]domain.PrecheckSnapshot{}, Authorizations: map[string]domain.AuthorizationManifest{}, Executions: map[string]domain.CutExecution{}, ExecutionReceipts: map[string]domain.ExecutionReceipt{}, VerificationAttempts: map[string][]domain.VerificationAttempt{}, FindingEvents: map[string][]domain.FindingEvent{}, Credentials: map[string]domain.ProvenanceCredential{}}
}
func (d *snapshot) init() {
	if d.Cores == nil {
		d.Cores = map[string]domain.CoreRecord{}
	}
	if d.CoreVersions == nil {
		d.CoreVersions = map[string][]domain.CoreVersion{}
	}
	if d.Cases == nil {
		d.Cases = map[string]domain.SamplingCase{}
	}
	if d.CaseVersions == nil {
		d.CaseVersions = map[string][]domain.CaseVersion{}
	}
	if d.Prechecks == nil {
		d.Prechecks = map[string]domain.PrecheckSnapshot{}
	}
	if d.Authorizations == nil {
		d.Authorizations = map[string]domain.AuthorizationManifest{}
	}
	if d.Executions == nil {
		d.Executions = map[string]domain.CutExecution{}
	}
	if d.ExecutionReceipts == nil {
		d.ExecutionReceipts = map[string]domain.ExecutionReceipt{}
	}
	if d.VerificationAttempts == nil {
		d.VerificationAttempts = map[string][]domain.VerificationAttempt{}
	}
	if d.FindingEvents == nil {
		d.FindingEvents = map[string][]domain.FindingEvent{}
	}
	if d.Credentials == nil {
		d.Credentials = map[string]domain.ProvenanceCredential{}
	}
}
func New(dir string) (*Store, error) {
	s := &Store{dir: dir, data: emptySnapshot(), verificationKeys: map[string]domain.VerificationAttempt{}}
	if dir != "" {
		if e := os.MkdirAll(dir, 0755); e != nil {
			return nil, e
		}
		b, e := os.ReadFile(filepath.Join(dir, "snapshot.json"))
		if e == nil {
			if e = json.Unmarshal(b, &s.data); e != nil {
				return nil, e
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	s.data.init()
	s.rebuildVerificationKeys()
	return s, nil
}
func clone(d snapshot) (snapshot, error) {
	b, e := json.Marshal(d)
	if e != nil {
		return snapshot{}, e
	}
	var out snapshot
	e = json.Unmarshal(b, &out)
	out.init()
	return out, e
}
func copyValue[T any](v T) T {
	b, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) persist(d snapshot) error {
	if s.dir == "" {
		return nil
	}
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.dir, "snapshot.tmp")
	if e = os.WriteFile(tmp, b, 0644); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(s.dir, "snapshot.json"))
}
func (s *Store) update(fn func(*snapshot) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, e := clone(s.data)
	if e != nil {
		return e
	}
	if e = fn(&next); e != nil {
		return e
	}
	next, e = clone(next)
	if e != nil {
		return e
	}
	if e = s.persist(next); e != nil {
		return e
	}
	s.data = next
	return nil
}
func (s *Store) Core(id string) (domain.CoreRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Cores[id]
	if !ok {
		return v, domain.ErrNotFound
	}
	return copyValue(v), nil
}
func (s *Store) Cores() []domain.CoreRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.CoreRecord, 0, len(s.data.Cores))
	for _, v := range s.data.Cores {
		out = append(out, copyValue(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CoreID < out[j].CoreID })
	return out
}
func (s *Store) SaveCore(c domain.CoreRecord) error {
	return s.update(func(d *snapshot) error { d.Cores[c.CoreID] = c; return nil })
}
func (s *Store) SaveCoreVersion(c domain.CoreRecord, v domain.CoreVersion) error {
	return s.update(func(d *snapshot) error {
		d.Cores[c.CoreID] = c
		d.CoreVersions[c.CoreID] = append(d.CoreVersions[c.CoreID], v)
		return nil
	})
}
func (s *Store) CoreVersions(id string) ([]domain.CoreVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data.Cores[id]; !ok {
		return nil, domain.ErrNotFound
	}
	out := copyValue(s.data.CoreVersions[id])
	sort.Slice(out, func(i, j int) bool { return out[i].Core.Revision < out[j].Core.Revision })
	return out, nil
}
func (s *Store) Case(id string) (domain.SamplingCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Cases[id]
	if !ok {
		return v, domain.ErrNotFound
	}
	return copyValue(v), nil
}
func (s *Store) Cases() []domain.SamplingCase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.SamplingCase, 0, len(s.data.Cases))
	for _, v := range s.data.Cases {
		out = append(out, copyValue(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CaseID < out[j].CaseID })
	return out
}
func (s *Store) SaveCase(c domain.SamplingCase) error {
	return s.update(func(d *snapshot) error { d.Cases[c.CaseID] = c; return nil })
}
func (s *Store) SaveCaseVersion(c domain.SamplingCase, v domain.CaseVersion) error {
	return s.update(func(d *snapshot) error {
		d.Cases[c.CaseID] = c
		d.CaseVersions[c.CaseID] = append(d.CaseVersions[c.CaseID], v)
		return nil
	})
}
func (s *Store) CaseVersions(id string) ([]domain.CaseVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data.Cases[id]; !ok {
		return nil, domain.ErrNotFound
	}
	out := copyValue(s.data.CaseVersions[id])
	sort.SliceStable(out, func(i, j int) bool { return out[i].Case.Revision < out[j].Case.Revision })
	return out, nil
}
func (s *Store) SavePrecheck(c domain.SamplingCase, p domain.PrecheckSnapshot) error {
	return s.update(func(d *snapshot) error {
		d.Cases[c.CaseID] = c
		if _, exists := d.Prechecks[p.Digest]; !exists {
			d.Prechecks[p.Digest] = p
		}
		return nil
	})
}
func (s *Store) Precheck(digest string) (domain.PrecheckSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Prechecks[digest]
	if !ok {
		return v, domain.ErrNotFound
	}
	return copyValue(v), nil
}
func (s *Store) SaveAuthorization(c domain.SamplingCase, m domain.AuthorizationManifest) error {
	return s.update(func(d *snapshot) error { d.Cases[c.CaseID] = c; d.Authorizations[c.CaseID] = m; return nil })
}
func (s *Store) Authorization(caseID string) (domain.AuthorizationManifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Authorizations[caseID]
	if !ok {
		return v, domain.ErrNotFound
	}
	return copyValue(v), nil
}
func receiptKey(caseID, key string) string { return caseID + "\x1f" + key }
func (s *Store) ExecutionReceipt(caseID, key string) (domain.ExecutionReceipt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.ExecutionReceipts[receiptKey(caseID, key)]
	return copyValue(v), ok
}
func (s *Store) Execution(caseID string) (domain.CutExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Executions[caseID]
	if !ok {
		return v, domain.ErrNotFound
	}
	return copyValue(v), nil
}
func (s *Store) CommitExecution(c domain.SamplingCase, v domain.CaseVersion, e domain.CutExecution, r domain.ExecutionReceipt) error {
	return s.update(func(d *snapshot) error {
		d.Cases[c.CaseID] = c
		d.CaseVersions[c.CaseID] = append(d.CaseVersions[c.CaseID], v)
		d.Executions[c.CaseID] = e
		d.ExecutionReceipts[receiptKey(c.CaseID, r.IdempotencyKey)] = r
		return nil
	})
}
func (s *Store) VerificationByKey(caseID, key string) (domain.VerificationAttempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.verificationKeys[receiptKey(caseID, key)]
	return copyValue(v), ok
}
func (s *Store) rebuildVerificationKeys() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verificationKeys = map[string]domain.VerificationAttempt{}
	for _, list := range s.data.VerificationAttempts {
		for _, a := range list {
			s.verificationKeys[receiptKey(a.CaseID, a.VerificationKey)] = copyValue(a)
		}
	}
}
func (s *Store) VerificationAttempts(caseID string) []domain.VerificationAttempt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyValue(s.data.VerificationAttempts[caseID])
}
func (s *Store) SaveRejectedVerification(a domain.VerificationAttempt) error {
	if err := s.update(func(d *snapshot) error {
		d.VerificationAttempts[a.CaseID] = append(d.VerificationAttempts[a.CaseID], a)
		return nil
	}); err != nil {
		return err
	}
	s.mu.Lock()
	s.verificationKeys[receiptKey(a.CaseID, a.VerificationKey)] = copyValue(a)
	s.mu.Unlock()
	return nil
}
func (s *Store) CommitFreeze(c domain.SamplingCase, v domain.CaseVersion, core domain.CoreRecord, cv domain.CoreVersion, a domain.VerificationAttempt, cred domain.ProvenanceCredential) error {
	if err := s.update(func(d *snapshot) error {
		d.Cases[c.CaseID] = c
		d.CaseVersions[c.CaseID] = append(d.CaseVersions[c.CaseID], v)
		d.Cores[core.CoreID] = core
		d.CoreVersions[core.CoreID] = append(d.CoreVersions[core.CoreID], cv)
		d.VerificationAttempts[c.CaseID] = append(d.VerificationAttempts[c.CaseID], a)
		d.Credentials[cred.CredentialID] = cred
		return nil
	}); err != nil {
		return err
	}
	s.mu.Lock()
	s.verificationKeys[receiptKey(c.CaseID, a.VerificationKey)] = copyValue(a)
	s.mu.Unlock()
	return nil
}
func (s *Store) SaveFindingChanges(c domain.SamplingCase, v domain.CaseVersion, events []domain.FindingEvent) error {
	return s.update(func(d *snapshot) error {
		d.Cases[c.CaseID] = c
		d.CaseVersions[c.CaseID] = append(d.CaseVersions[c.CaseID], v)
		d.FindingEvents[c.CaseID] = append(d.FindingEvents[c.CaseID], events...)
		return nil
	})
}
func (s *Store) SaveFindingEvents(c domain.SamplingCase, events []domain.FindingEvent) error {
	return s.update(func(d *snapshot) error {
		d.Cases[c.CaseID] = c
		d.FindingEvents[c.CaseID] = append(d.FindingEvents[c.CaseID], events...)
		return nil
	})
}
func (s *Store) FindingEvents(caseID string) []domain.FindingEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyValue(s.data.FindingEvents[caseID])
}
func (s *Store) Credential(id string) (domain.ProvenanceCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data.Credentials[id]
	if !ok {
		return v, domain.ErrNotFound
	}
	return copyValue(v), nil
}
func (s *Store) SaveCredential(c domain.ProvenanceCredential) error {
	return s.update(func(d *snapshot) error { d.Credentials[c.CredentialID] = c; return nil })
}
func (s *Store) SaveIdempotency(k string, e domain.CutExecution) error {
	return s.update(func(d *snapshot) error {
		r := domain.ExecutionReceipt{CaseID: e.CaseID, IdempotencyKey: k, RequestDigest: e.RequestDigest, ExecutionID: e.ExecutionID, Status: domain.Executed, Execution: e, CreatedAt: e.ExecutedAt}
		d.ExecutionReceipts[receiptKey(e.CaseID, k)] = r
		return nil
	})
}
func (s *Store) Idempotency(k string) (domain.CutExecution, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.data.ExecutionReceipts {
		if r.IdempotencyKey == k {
			return copyValue(r.Execution), true
		}
	}
	return domain.CutExecution{}, false
}
