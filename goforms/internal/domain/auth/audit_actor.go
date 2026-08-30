package auth

import (
	"encoding/base64"
	"errors"
	"regexp"

	"github.com/google/uuid"
)

type CredentialClass string

const (
	CredentialClassServiceToken        CredentialClass = "service_token"
	CredentialClassFirstPartyAssertion CredentialClass = "first_party_assertion"
	CredentialClassDatabaseOperator    CredentialClass = "database_operator"
)

var auditIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$`)

// AuditActor is supplied explicitly by a trusted authenticated boundary, never
// inferred from a request body or an optional context fallback.
type AuditActor struct {
	OrganizationID  string
	SubjectID       string
	CredentialClass CredentialClass
	CredentialID    string
	RequestID       string
}

func (a AuditActor) Validate() error {
	invalid := errors.New("invalid management audit actor")
	if _, err := uuid.Parse(a.OrganizationID); err != nil {
		return invalid
	}
	for _, id := range []string{a.SubjectID, a.CredentialID, a.RequestID} {
		if !auditIdentityPattern.MatchString(id) {
			return invalid
		}
	}
	switch a.CredentialClass {
	case CredentialClassServiceToken, CredentialClassDatabaseOperator:
		if a.SubjectID != a.CredentialID {
			return invalid
		}
	case CredentialClassFirstPartyAssertion:
	default:
		return invalid
	}
	return nil
}

// DatabaseAuditActor must receive current_user from the authenticated database
// connection. This identifies a database role, not a purported human operator.
func DatabaseAuditActor(role, organizationID string) AuditActor {
	if role == "" {
		return AuditActor{}
	}
	id := "db:" + base64.RawURLEncoding.EncodeToString([]byte(role))
	return AuditActor{OrganizationID: organizationID, SubjectID: id,
		CredentialClass: CredentialClassDatabaseOperator, CredentialID: id, RequestID: uuid.NewString()}
}
