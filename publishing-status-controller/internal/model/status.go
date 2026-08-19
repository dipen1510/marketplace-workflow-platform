package model

import "time"

type Phase string

const (
	PhaseUnknown   Phase = "UNKNOWN"
	PhasePending   Phase = "PENDING"
	PhaseRunning   Phase = "RUNNING"
	PhaseSucceeded Phase = "SUCCEEDED"
	PhaseFailed    Phase = "FAILED"
	PhaseError     Phase = "ERROR"
	PhaseSkipped   Phase = "SKIPPED"
	PhaseOmitted   Phase = "OMITTED"
)

type JobStatus struct {
	WorkflowUID     string `json:"workflowUid"`
	WorkflowName    string `json:"workflowName"`
	Namespace       string `json:"namespace"`
	ResourceVersion string `json:"resourceVersion"`

	Phase   Phase  `json:"phase"`
	Message string `json:"message,omitempty"`

	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	Steps []StepStatus `json:"steps"`
}

type StepStatus struct {
	NodeID string `json:"nodeId"`
	Name   string `json:"name"`

	Phase   Phase  `json:"phase"`
	Message string `json:"message,omitempty"`

	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	Attempts int `json:"attempts"`
}
