package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	wfv1 "github.com/argoproj/argo-workflows/v4/pkg/apis/workflow/v1alpha1"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/metrics"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/model"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/publishing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const maxRetries = 8

type StatusPublisher interface {
	UpdateJobStatus(
		ctx context.Context,
		job model.JobStatus,
	) error
}

type Controller struct {
	informer  cache.SharedIndexInformer
	queue     workqueue.TypedRateLimitingInterface[string]
	publisher StatusPublisher
	metrics   *metrics.Recorder
}

func New(informer cache.SharedIndexInformer, publisher StatusPublisher, recorder *metrics.Recorder) (*Controller, error) {
	rateLimiter :=
		workqueue.
			NewTypedItemExponentialFailureRateLimiter[string](
			1*time.Second,
			30*time.Second,
		)

	queue :=
		workqueue.NewTypedRateLimitingQueue(
			rateLimiter,
		)
	c := &Controller{
		informer:  informer,
		queue:     queue,
		publisher: publisher,
		metrics:   recorder,
	}

	_, err := informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addWorkflow,
			UpdateFunc: c.updateWorkflow,
			DeleteFunc: c.deleteWorkflow,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("register workflow event handler: %w", err)
	}
	return c, nil
}

func (c *Controller) addWorkflow(obj interface{}) {
	workflow, ok := obj.(*wfv1.Workflow)
	if !ok {
		return
	}
	c.metrics.Event("add")

	fmt.Printf(
		"[Event ADD] workflow=%s namespace=%s phase=%s\n",
		workflow.Name,
		workflow.Namespace,
		workflow.Status.Phase,
	)
	c.enqueue(obj)
}

func (c *Controller) updateWorkflow(
	oldObj interface{},
	newObj interface{},
) {
	oldWorkflow, ok := oldObj.(*wfv1.Workflow)
	if !ok {
		return
	}

	newWorkflow, ok := newObj.(*wfv1.Workflow)
	if !ok {
		return
	}
	c.metrics.Event("update")

	fmt.Printf(
		"[EVENT UPDATE] workflow=%s phase=%s -> %s\n",
		newWorkflow.Name,
		oldWorkflow.Status.Phase,
		newWorkflow.Status.Phase,
	)
	c.enqueue(newObj)
}

func (c *Controller) deleteWorkflow(obj interface{}) {
	key, err :=
		cache.DeletionHandlingMetaNamespaceKeyFunc(
			obj,
		)

	if err != nil {
		fmt.Printf(
			"[EVENT DELETE] failed to build key: %v\n",
			err,
		)

		return
	}
	c.metrics.Event("delete")

	fmt.Printf(
		"[EVENT DELETE] key=%s\n",
		key,
	)

	c.queue.Add(key)
	c.metrics.SetQueueDepth(
		c.queue.Len(),
	)
}

func (c *Controller) enqueue(
	obj interface{},
) {

	key, err :=
		cache.MetaNamespaceKeyFunc(obj)

	if err != nil {

		fmt.Printf(
			"failed to create workflow key: %v\n",
			err,
		)

		return
	}

	fmt.Printf(
		"[QUEUE ADD] key=%s\n",
		key,
	)

	c.queue.Add(key)
	c.metrics.SetQueueDepth(
		c.queue.Len(),
	)
}

func (c *Controller) Run(
	ctx context.Context,
	workers int,
) {

	fmt.Printf(
		"Starting status controller workers=%d\n",
		workers,
	)

	defer func() {

		fmt.Println(
			"Shutting down status controller queue",
		)

		c.queue.ShutDown()
		c.metrics.SetQueueDepth(
			c.queue.Len(),
		)
	}()

	for i := 0; i < workers; i++ {

		workerID := i + 1

		go c.runWorker(
			ctx,
			workerID,
		)
	}

	<-ctx.Done()
}

func (c *Controller) runWorker(
	ctx context.Context,
	workerID int,
) {

	fmt.Printf(
		"[WORKER %d] started\n",
		workerID,
	)

	for c.processNextWorkItem(
		ctx,
		workerID,
	) {
	}
}

func (c *Controller) processNextWorkItem(
	ctx context.Context,
	workerID int,
) bool {

	key, shutdown :=
		c.queue.Get()

	if shutdown {
		c.metrics.SetQueueDepth(
			c.queue.Len(),
		)

		fmt.Printf(
			"[WORKER %d] queue shutdown\n",
			workerID,
		)

		return false
	}

	defer c.queue.Done(key)

	fmt.Printf(
		"[WORKER %d] processing key=%s\n",
		workerID,
		key,
	)

	err :=
		c.syncWorkflow(
			ctx,
			key,
		)

	if err == nil {

		c.queue.Forget(key)
		c.metrics.Sync(
			"success",
		)

		fmt.Printf(
			"[WORKER %d] success key=%s\n",
			workerID,
			key,
		)

		return true
	}

	// If we reached here, syncWorkflow failed.
	// Count every failed synchronization attempt once.
	c.metrics.Sync(
		"failure",
	)

	var httpErr *publishing.HTTPError

	if errors.As(
		err,
		&httpErr,
	) &&
		!httpErr.Retryable() {

		c.metrics.Dropped()

		c.queue.Forget(key)

		fmt.Printf(
			"[WORKER %d] permanent sync failure key=%s status=%d error=%v\n",
			workerID,
			key,
			httpErr.StatusCode,
			err,
		)

		return true
	}

	// Temporary failure:
	// retry using queue rate limiter.
	if c.queue.NumRequeues(key) < maxRetries {

		fmt.Printf(
			"[WORKER %d] sync failed key=%s retry=%d error=%v\n",
			workerID,
			key,
			c.queue.NumRequeues(key)+1,
			err,
		)
		c.metrics.Retry()

		c.queue.
			AddRateLimited(
				key,
			)

		return true
	}

	// Too many failures.
	c.metrics.Dropped()
	c.queue.Forget(key)

	fmt.Printf(
		"[WORKER %d] dropping key=%s after %d retries error=%v\n",
		workerID,
		key,
		maxRetries,
		err,
	)

	return true
}

func (c *Controller) syncWorkflow(
	ctx context.Context,
	key string,
) error {

	_ = ctx

	obj, exists, err :=
		c.informer.
			GetIndexer().
			GetByKey(key)

	if err != nil {

		return fmt.Errorf(
			"get workflow %s from informer cache: %w",
			key,
			err,
		)
	}

	if !exists {

		// Workflow may have been deleted
		// after its key entered the queue.
		fmt.Printf(
			"[SYNC] workflow no longer exists key=%s\n",
			key,
		)

		return nil
	}

	workflow, ok :=
		obj.(*wfv1.Workflow)

	if !ok {

		return fmt.Errorf(
			"cached object for %s is not an Argo Workflow",
			key,
		)
	}

	jobStatus :=
		buildJobStatus(
			workflow,
		)

	payload, err :=
		json.MarshalIndent(
			jobStatus,
			"",
			"  ",
		)

	if err != nil {

		return fmt.Errorf(
			"marshal job status for %s: %w",
			key,
			err,
		)
	}

	fmt.Printf(
		"[SYNC JOB]\n%s\n",
		string(payload),
	)

	if err :=
		c.publisher.UpdateJobStatus(
			ctx,
			jobStatus,
		); err != nil {

		return fmt.Errorf(
			"synchronize workflow %s with Marketplace Publishing: %w",
			key,
			err,
		)
	}

	return nil
}
