package publishing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/metrics"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	metrics    *metrics.Recorder
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf(
		"publishing status API returned HTTP %d: %s",
		e.StatusCode,
		e.Body,
	)
}

func (e *HTTPError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests ||
		e.StatusCode >= http.StatusInternalServerError
}

func NewClient(
	baseURL string,
	timeout time.Duration,
	recorder *metrics.Recorder,
) (*Client, error) {

	parsed, err := url.Parse(baseURL)
	if err != nil ||
		parsed.Scheme == "" ||
		parsed.Host == "" {

		return nil, fmt.Errorf(
			"invalid publishing base URL %q",
			baseURL,
		)
	}

	return &Client{
		baseURL: strings.TrimRight(
			baseURL,
			"/",
		),

		httpClient: &http.Client{
			Timeout: timeout,
		},

		metrics: recorder,
	}, nil
}

func (c *Client) UpdateJobStatus(
	ctx context.Context,
	job model.JobStatus,
) error {

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf(
			"marshal workflow status %s: %w",
			job.WorkflowName,
			err,
		)
	}

	endpoint := fmt.Sprintf(
		"%s/v1/workflows/%s/status",
		c.baseURL,
		url.PathEscape(job.WorkflowUID),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		endpoint,
		bytes.NewReader(payload),
	)

	if err != nil {
		return fmt.Errorf(
			"create publishing status request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	started := time.Now()

	resp, err := c.httpClient.Do(req)

	if err != nil {
		c.metrics.ObserveHTTPRequest(
			"transport_error",
			time.Since(started),
		)

		return fmt.Errorf(
			"update workflow status %s: %w",
			job.WorkflowName,
			err,
		)
	}

	defer resp.Body.Close()

	statusClass := fmt.Sprintf(
		"%dxx",
		resp.StatusCode/100,
	)

	c.metrics.ObserveHTTPRequest(
		statusClass,
		time.Since(started),
	)

	if resp.StatusCode >= 200 &&
		resp.StatusCode < 300 {

		fmt.Printf(
			"[REST] synchronized workflow=%s phase=%s resourceVersion=%s\n",
			job.WorkflowName,
			job.Phase,
			job.ResourceVersion,
		)

		return nil
	}

	body, _ := io.ReadAll(
		io.LimitReader(
			resp.Body,
			4096,
		),
	)

	return &HTTPError{
		StatusCode: resp.StatusCode,
		Body: strings.TrimSpace(
			string(body),
		),
	}
}
