package publishing

import (
	"context"
	"fmt"
	"sync"

	publishingv1 "github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/api"
)

type Service struct {
	publishingv1.UnimplementedMarketplacePublishingServiceServer

	mu sync.RWMutex

	jobs map[string]*publishingv1.JobStatus
}

func NewService() *Service {
	return &Service{
		jobs: make(
			map[string]*publishingv1.JobStatus,
		),
	}
}

func (s *Service) UpdateJobStatus(
	ctx context.Context,
	req *publishingv1.UpdateJobStatusRequest,
) (
	*publishingv1.UpdateJobStatusResponse,
	error,
) {
	_ = ctx

	if req.GetJob() == nil {
		return nil,
			fmt.Errorf(
				"job status is required",
			)
	}

	job := req.GetJob()

	if job.GetWorkflowUid() == "" {
		return nil,
			fmt.Errorf(
				"workflow UID is required",
			)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists :=
		s.jobs[job.GetWorkflowUid()]

	if exists {

		fmt.Printf(
			"[PUBLISHING] UPDATE workflow=%s phase=%s -> %s resourceVersion=%s\n",
			job.GetWorkflowName(),
			existing.GetPhase(),
			job.GetPhase(),
			job.GetResourceVersion(),
		)

	} else {

		fmt.Printf(
			"[PUBLISHING] CREATE workflow=%s phase=%s resourceVersion=%s\n",
			job.GetWorkflowName(),
			job.GetPhase(),
			job.GetResourceVersion(),
		)
	}

	//
	// Phase 1:
	// latest complete snapshot replaces previous snapshot.
	//
	s.jobs[job.GetWorkflowUid()] = job

	printSteps(job)

	return &publishingv1.UpdateJobStatusResponse{
		Updated: true,
	}, nil
}

func printSteps(
	job *publishingv1.JobStatus,
) {

	for _, step := range job.GetSteps() {

		fmt.Printf(
			"    step=%s phase=%s attempts=%d\n",
			step.GetName(),
			step.GetPhase(),
			step.GetAttempts(),
		)
	}
}
