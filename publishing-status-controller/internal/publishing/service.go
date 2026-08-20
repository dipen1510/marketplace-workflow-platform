package publishing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/model"
)

type Service struct {
	mu sync.RWMutex

	jobs map[string]model.JobStatus
}

func NewService() *Service {
	return &Service{
		jobs: make(map[string]model.JobStatus),
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"/v1/workflows/",
		s.handleWorkflowStatus,
	)

	return mux
}

func (s *Service) handleWorkflowStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPut {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	if !strings.HasSuffix(
		r.URL.Path,
		"/status",
	) {
		http.NotFound(w, r)
		return
	}

	var job model.JobStatus

	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if job.WorkflowUID == "" {
		http.Error(
			w,
			"workflowUid is required",
			http.StatusBadRequest,
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists :=
		s.jobs[job.WorkflowUID]

	if exists {
		fmt.Printf(
			"[PUBLISHING] UPDATE workflow=%s phase=%s -> %s resourceVersion=%s\n",
			job.WorkflowName,
			existing.Phase,
			job.Phase,
			job.ResourceVersion,
		)
	} else {
		fmt.Printf(
			"[PUBLISHING] CREATE workflow=%s phase=%s resourceVersion=%s\n",
			job.WorkflowName,
			job.Phase,
			job.ResourceVersion,
		)
	}

	s.jobs[job.WorkflowUID] = job

	for _, step := range job.Steps {
		fmt.Printf(
			"    step=%s phase=%s attempts=%d\n",
			step.Name,
			step.Phase,
			step.Attempts,
		)
	}

	w.Header().
		Set(
			"Content-Type",
			"application/json",
		)

	w.WriteHeader(
		http.StatusOK,
	)

	_, _ = w.Write(
		[]byte(`{"updated":true}`),
	)
}
