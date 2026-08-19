package publishing

import (
	"time"

	publishingv1 "github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/api"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoJob(
	job model.JobStatus,
) *publishingv1.JobStatus {

	steps :=
		make(
			[]*publishingv1.StepStatus,
			0,
			len(job.Steps),
		)

	for _, step := range job.Steps {

		steps =
			append(
				steps,
				toProtoStep(step),
			)
	}

	return &publishingv1.JobStatus{
		WorkflowUid: job.WorkflowUID,

		WorkflowName: job.WorkflowName,

		Namespace: job.Namespace,

		ResourceVersion: job.ResourceVersion,

		Phase: toProtoPhase(job.Phase),

		Message: job.Message,

		StartedAt: toProtoTime(job.StartedAt),

		FinishedAt: toProtoTime(job.FinishedAt),

		Steps: steps,
	}
}

func toProtoStep(
	step model.StepStatus,
) *publishingv1.StepStatus {

	return &publishingv1.StepStatus{
		NodeId: step.NodeID,

		Name: step.Name,

		Phase: toProtoPhase(step.Phase),

		Message: step.Message,

		StartedAt: toProtoTime(step.StartedAt),

		FinishedAt: toProtoTime(step.FinishedAt),

		Attempts: int32(step.Attempts),
	}
}

func toProtoPhase(
	phase model.Phase,
) publishingv1.JobPhase {

	switch phase {

	case model.PhasePending:

		return publishingv1.
			JobPhase_JOB_PHASE_PENDING

	case model.PhaseRunning:

		return publishingv1.
			JobPhase_JOB_PHASE_RUNNING

	case model.PhaseSucceeded:

		return publishingv1.
			JobPhase_JOB_PHASE_SUCCEEDED

	case model.PhaseFailed:

		return publishingv1.
			JobPhase_JOB_PHASE_FAILED

	case model.PhaseError:

		return publishingv1.
			JobPhase_JOB_PHASE_ERROR

	case model.PhaseSkipped:

		return publishingv1.
			JobPhase_JOB_PHASE_SKIPPED

	case model.PhaseOmitted:

		return publishingv1.
			JobPhase_JOB_PHASE_OMITTED

	default:

		return publishingv1.
			JobPhase_JOB_PHASE_UNSPECIFIED
	}
}

func toProtoTime(
	value *time.Time,
) *timestamppb.Timestamp {

	if value == nil {
		return nil
	}

	return timestamppb.New(
		value.UTC(),
	)
}
