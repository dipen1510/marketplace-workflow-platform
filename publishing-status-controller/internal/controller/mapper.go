package controller

import (
	"sort"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v4/pkg/apis/workflow/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/model"
)

func buildJobStatus(
	workflow *wfv1.Workflow,
) model.JobStatus {

	job := model.JobStatus{
		WorkflowUID:     string(workflow.UID),
		WorkflowName:    workflow.Name,
		Namespace:       workflow.Namespace,
		ResourceVersion: workflow.ResourceVersion,

		Phase:   mapPhase(string(workflow.Status.Phase)),
		Message: workflow.Status.Message,

		StartedAt:  toTimePtr(workflow.Status.StartedAt),
		FinishedAt: toTimePtr(workflow.Status.FinishedAt),
	}

	job.Steps =
		extractLogicalSteps(
			workflow.Status.Nodes,
		)

	return job
}

func extractLogicalSteps(
	nodes wfv1.Nodes,
) []model.StepStatus {

	var steps []model.StepStatus

	/*
		Retry nodes look conceptually like:

		validate (Retry)
		   |
		   +-- attempt 0 Pod
		   +-- attempt 1 Pod

		We want:

		validate
		   phase = Running
		   attempts = 2

		NOT:

		validate(0) FAILED
		validate(1) RUNNING
	*/

	retryChildren :=
		make(map[string]struct{})

	for _, node := range nodes {

		if node.Type != wfv1.NodeTypeRetry {
			continue
		}

		for _, childID := range node.Children {

			retryChildren[childID] =
				struct{}{}
		}
	}

	for _, node := range nodes {

		switch node.Type {

		case wfv1.NodeTypeRetry:

			steps =
				append(
					steps,
					mapRetryNode(node),
				)

		case wfv1.NodeTypePod:

			// If this Pod belongs to a Retry node,
			// don't expose it as another logical step.
			if _, isRetryAttempt :=
				retryChildren[node.ID]; isRetryAttempt {

				continue
			}

			steps =
				append(
					steps,
					mapPodNode(node),
				)
		}
	}

	sortSteps(steps)

	return steps
}

func mapPodNode(
	node wfv1.NodeStatus,
) model.StepStatus {

	return model.StepStatus{
		NodeID: node.ID,

		Name: logicalStepName(node),

		Phase: mapPhase(
			string(node.Phase),
		),

		Message: node.Message,

		StartedAt: toTimePtr(
			node.StartedAt,
		),

		FinishedAt: toTimePtr(
			node.FinishedAt,
		),

		Attempts: 1,
	}
}

func mapRetryNode(
	node wfv1.NodeStatus,
) model.StepStatus {

	return model.StepStatus{
		NodeID: node.ID,

		Name: logicalStepName(node),

		Phase: mapPhase(
			string(node.Phase),
		),

		Message: node.Message,

		StartedAt: toTimePtr(
			node.StartedAt,
		),

		FinishedAt: toTimePtr(
			node.FinishedAt,
		),

		Attempts: len(node.Children),
	}
}

func logicalStepName(
	node wfv1.NodeStatus,
) string {

	if node.DisplayName != "" {
		return node.DisplayName
	}

	if node.TemplateName != "" {
		return node.TemplateName
	}

	return node.Name
}

func mapPhase(
	argoPhase string,
) model.Phase {

	switch argoPhase {

	case "Pending":
		return model.PhasePending

	case "Running":
		return model.PhaseRunning

	case "Succeeded":
		return model.PhaseSucceeded

	case "Failed":
		return model.PhaseFailed

	case "Error":
		return model.PhaseError

	case "Skipped":
		return model.PhaseSkipped

	case "Omitted":
		return model.PhaseOmitted

	default:
		return model.PhaseUnknown
	}
}

func toTimePtr(
	t metav1.Time,
) *time.Time {

	if t.Time.IsZero() {
		return nil
	}

	value := t.Time.UTC()

	return &value
}

func sortSteps(
	steps []model.StepStatus,
) {

	sort.Slice(
		steps,
		func(i, j int) bool {

			left :=
				steps[i].StartedAt

			right :=
				steps[j].StartedAt

			// Neither started.
			if left == nil &&
				right == nil {

				return steps[i].Name <
					steps[j].Name
			}

			// Started steps come before
			// not-yet-started steps.
			if left == nil {
				return false
			}

			if right == nil {
				return true
			}

			if left.Equal(*right) {

				return steps[i].Name <
					steps[j].Name
			}

			return left.Before(*right)
		},
	)
}
