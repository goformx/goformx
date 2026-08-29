package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SubmissionStatus string

const SubmissionStatusAccepted SubmissionStatus = "accepted"

// FormSubmission is an immutable accepted payload tied to an exact schema version.
// Delivery progress belongs to the webhook outbox, not to this record.
type FormSubmission struct {
	ID             string           `gorm:"column:uuid;primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	FormID         string           `gorm:"not null;index;type:uuid"                                   json:"form_id"`
	SchemaVersion  int              `gorm:"not null"                                                   json:"schema_version"`
	RequestID      string           `gorm:"column:request_id;not null"                                 json:"request_id"`
	IdempotencyKey string           `gorm:"column:idempotency_key"                                     json:"-"`
	Data           JSON             `gorm:"type:jsonb;not null"                                        json:"data"`
	SubmittedAt    time.Time        `gorm:"not null"                                                   json:"submitted_at"`
	Status         SubmissionStatus `gorm:"not null;size:20"                                           json:"status"`
	Metadata       JSON             `gorm:"type:jsonb"                                                 json:"metadata"`
	CreatedAt      time.Time        `gorm:"not null;autoCreateTime"                                    json:"created_at"`
	UpdatedAt      time.Time        `gorm:"not null;autoUpdateTime"                                    json:"updated_at"`
}

func (*FormSubmission) TableName() string { return "form_submissions" }

func (s *FormSubmission) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.Status == "" {
		s.Status = SubmissionStatusAccepted
	}
	if s.RequestID == "" {
		s.RequestID = "req_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return nil
}
