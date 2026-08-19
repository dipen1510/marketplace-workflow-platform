package publishing

import (
	"context"
	"fmt"
	"time"

	publishingv1 "github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/api"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn *grpc.ClientConn

	client publishingv1.
		MarketplacePublishingServiceClient

	timeout time.Duration
}

func NewClient(
	address string,
	timeout time.Duration,
) (*Client, error) {

	conn, err :=
		grpc.NewClient(
			address,
			grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			),
		)

	if err != nil {

		return nil,
			fmt.Errorf(
				"create publishing gRPC client: %w",
				err,
			)
	}

	return &Client{
		conn: conn,

		client: publishingv1.
			NewMarketplacePublishingServiceClient(
				conn,
			),

		timeout: timeout,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) UpdateJobStatus(
	ctx context.Context,
	job model.JobStatus,
) error {

	rpcCtx, cancel :=
		context.WithTimeout(
			ctx,
			c.timeout,
		)

	defer cancel()

	request :=
		&publishingv1.
			UpdateJobStatusRequest{
			Job: toProtoJob(job),
		}

	response, err :=
		c.client.
			UpdateJobStatus(
				rpcCtx,
				request,
			)

	if err != nil {

		return fmt.Errorf(
			"UpdateJobStatus workflow=%s: %w",
			job.WorkflowName,
			err,
		)
	}

	if !response.GetUpdated() {

		return fmt.Errorf(
			"publishing service did not update workflow=%s",
			job.WorkflowName,
		)
	}

	fmt.Printf(
		"[GRPC] synchronized workflow=%s phase=%s resourceVersion=%s\n",
		job.WorkflowName,
		job.Phase,
		job.ResourceVersion,
	)

	return nil
}
