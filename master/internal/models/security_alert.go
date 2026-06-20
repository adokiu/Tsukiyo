package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

type AlertStatus string

const (
	AlertStatusOpen     AlertStatus = "open"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusIgnored  AlertStatus = "ignored"
)

type SecurityAlert struct {
	ID         uuid.UUID     `json:"id" gorm:"type:uuid;primaryKey"`
	NodeID     uuid.UUID     `json:"node_id" gorm:"type:uuid;index;not null"`
	InstanceID string        `json:"instance_id,omitempty" gorm:"size:64;index"`
	AlertType  string        `json:"alert_type" gorm:"size:50;not null;index"`
	Severity   AlertSeverity `json:"severity" gorm:"size:20;not null;index"`
	Status     AlertStatus   `json:"status" gorm:"size:20;not null;default:'open';index"`
	SourceIP   string        `json:"source_ip,omitempty" gorm:"size:45"`
	DestPort   int           `json:"dest_port,omitempty"`
	Protocol   string        `json:"protocol,omitempty" gorm:"size:10"`
	Details    string        `json:"details" gorm:"type:text;not null"`
	RawData    string        `json:"raw_data,omitempty" gorm:"type:text"`
	AutoAction string        `json:"auto_action,omitempty" gorm:"size:50"`
	ResolvedBy *uint         `json:"resolved_by,omitempty"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
	DetectedAt time.Time     `json:"detected_at" gorm:"not null;index"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (a *SecurityAlert) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Status == "" {
		a.Status = AlertStatusOpen
	}
	return nil
}
