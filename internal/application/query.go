package application

import "corepreservation/internal/domain"

func (s *Service) CaseHistory(coreID string) []domain.SamplingCase {
	var out []domain.SamplingCase
	for _, c := range s.st.Cases() {
		if c.CoreID == coreID {
			out = append(out, c)
		}
	}
	return out
}
func (s *Service) CredentialByCase(caseID string) (domain.ProvenanceCredential, error) {
	c, e := s.st.Case(caseID)
	if e != nil || c.Credential == nil {
		return domain.ProvenanceCredential{}, domain.ErrNotFound
	}
	return *c.Credential, nil
}
