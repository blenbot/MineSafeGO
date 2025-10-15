package models

import (
	"errors"
	"time"
)

type MediaStatus string
type ResolutionStatus string

const (
	StatusSynced        MediaStatus = "SYNCED"
	StatusPendingUpload MediaStatus = "PENDING_UPLOAD"
	StatusNotApplicable MediaStatus = "NOT_APPLICABLE"
	StatusUploadFailed  MediaStatus = "UPLOAD_FAILED"
)

const (
	ResolutionPending   ResolutionStatus = "PENDING"
	ResolutionComplete  ResolutionStatus = "RESOLVED"
	ResolutionActive    ResolutionStatus = "RESOLVING"
	ResolutionCancelled ResolutionStatus = "CANCELLED"
)

type Emergency struct {
	ID                    int              `json:"id" db:"id"`
	UserID                string           `json:"user_id" db:"user_id"`
	EmergencyID           int              `json:"emergency_id" db:"emergency_id"`
	Severity              string           `json:"severity" db:"severity"`
	Lat                   float64          `json:"latitude" db:"latitude"`
	Lon                   float64          `json:"longitude" db:"longitude"`
	Issue                 string           `json:"issue" db:"issue"`
	MediaStatus           MediaStatus      `json:"media_status" db:"media_status"`
	MediaURL              *string          `json:"media_url,omitempty" db:"media_url"`
	Location              *string          `json:"location,omitempty" db:"location"`
	IncidentTime          *time.Time       `json:"incident_time,omitempty" db:"incident_time"`
	IncidentReportingTime time.Time        `json:"reporting_time" db:"reporting_time"`
	Status                ResolutionStatus `json:"status" db:"status"`
	ResolutionTime        *time.Time       `json:"resolution_time,omitempty" db:"resolution_time"`
}

type EmergencyCreate struct {
	UserID      string      `json:"user_id"`
	EmergencyID int         `json:"emergency_id"`
	Severity    string      `json:"severity"`
	Latitude    float64     `json:"latitude"`
	Longitude   float64     `json:"longitude"`
	Issue       string      `json:"issue"`
	MediaStatus MediaStatus `json:"media_status,omitempty"`
}

func NewEmergency(userID string, emergencyID int, severity string, lat, lon float64, issue string, mediaStatus MediaStatus, mediaURL *string, incidentTime *time.Time) (*Emergency, error) {
	if userID == "" {
		return nil, errors.New("invalid UserID")
	}

	if mediaStatus == "" {
		mediaStatus = StatusNotApplicable
	}

	em := &Emergency{
		UserID:                userID,
		EmergencyID:           emergencyID,
		Severity:              severity,
		Lat:                   lat,
		Lon:                   lon,
		Issue:                 issue,
		MediaStatus:           mediaStatus,
		MediaURL:              mediaURL,
		IncidentTime:          incidentTime,
		IncidentReportingTime: time.Now().UTC(),
		Status:                ResolutionPending,
	}
	return em, nil
}
