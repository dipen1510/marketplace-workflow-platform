# Marketplace Workflow Platform

A production-style workflow orchestration platform built with **Go**, **Kubernetes**, **Kubebuilder**, and **Argo Workflows**.

The project demonstrates how a custom Kubernetes controller can provide a domain-specific workflow API while delegating workflow scheduling and execution to Argo Workflows.

The controller manages the desired state of Marketplace workflows, while Argo handles scheduling, DAG execution, retries, concurrency, and worker pods.

---

## Architecture

```text
                       Kubernetes API Server
                                |
                                |
                       MarketplaceWorkflow
                         Custom Resource
                                |
                                | watch event
                                v
                     controller-runtime queue
                                |
                                v
                         Go Controller
                         Reconcile()
                                |
                    +-----------+------------+
                    |                        |
                    | validate               | create/update
                    v                        v
             WorkflowTemplate          CronWorkflow
                (Argo)                    (Argo)
                                             |
                                             | cron schedule
                                             |
                                             v
                                         Workflow
                                      one execution/run
                                             |
                                             v
                                      Argo DAG / Steps
                                             |
                                  +----------+----------+
                                  |          |          |
                                  v          v          v
                              discover    validate    publish
                                  |          |          |
                                  +----------+----------+
                                             |
                                             v
                                      Go Worker Pods
```

---

## Core Design

There are four important Kubernetes resources in the architecture.

### MarketplaceWorkflow

`MarketplaceWorkflow` is the domain-specific Custom Resource managed by this project.

Example:

```yaml
apiVersion: workflow.marketplace.example.com/v1alpha1
kind: MarketplaceWorkflow

metadata:
  name: publishing
  namespace: argo

spec:
  schedules:
    - "*/5 * * * *"

  timezone: America/Los_Angeles

  concurrencyPolicy: Forbid

  suspend: false

  workflowTemplateRef:
    name: marketplace-publishing

  parameters:
    - name: failureMode
      value: success
```

It describes **what Marketplace wants**, not how individual workflow steps are executed.

---

### WorkflowTemplate

The Argo `WorkflowTemplate` contains the reusable workflow implementation.

For Publishing:

```text
discover
   |
   v
validate
   |
   v
publish
```

The template owns execution-specific configuration such as:

- DAG dependencies
- container images
- commands
- retry strategies
- retry backoff
- parameters
- timeouts

This keeps execution logic out of the custom Kubernetes controller.

---

### CronWorkflow

The custom controller reconciles a `MarketplaceWorkflow` into an Argo `CronWorkflow`.

```text
MarketplaceWorkflow
        |
        | Reconcile()
        v
CronWorkflow
```

For example:

```text
MarketplaceWorkflow:
schedule = */5 * * * *

             |
             v

CronWorkflow:
schedule = */5 * * * *
```

The controller continuously ensures that the Argo resource matches the desired Marketplace configuration.

---

### Workflow

A `Workflow` represents **one execution**.

For a workflow running every five minutes:

```text
CronWorkflow/publishing
          |
          +---- 10:00 → Workflow/publishing-a1
          |
          +---- 10:05 → Workflow/publishing-b2
          |
          +---- 10:10 → Workflow/publishing-c3
```

Argo, not the custom controller, creates these Workflow objects.

---

## Controller Reconciliation

The controller uses the standard Kubernetes reconciliation model.

```text
MarketplaceWorkflow event
          |
          v
controller-runtime
     work queue
          |
          v
Reconcile(namespace/name)
          |
          v
Get MarketplaceWorkflow
          |
          v
Verify WorkflowTemplate
          |
     +----+----+
     |         |
 missing      exists
     |         |
     v         v
 NotReady   Build desired
             CronWorkflow
                  |
                  v
          Create or Update
                  |
                  v
             syncStatus()
```

The controller does **not** execute workflow steps.

Its responsibility is maintaining desired state.

Argo owns execution.

---

## Reconciliation Queue

`controller-runtime` watches Kubernetes resources and automatically maintains a work queue.

For example:

```text
MarketplaceWorkflow/publishing updated
MarketplaceWorkflow/promotion updated
MarketplaceWorkflow/transfer updated

                   |
                   v

          controller-runtime

             Work Queue

        +---------------------+
        | argo/publishing     |
        | argo/promotion      |
        | argo/transfer       |
        +---------------------+

                   |
                   v

              Reconcile()
```

Each `Reconcile()` invocation receives one resource key:

```go
Request{
    Namespace: "argo",
    Name:      "publishing",
}
```

The reconciler then reads the latest state from Kubernetes.

---

## Self-Healing

The controller continuously restores desired state.

For example, suppose Marketplace declares:

```text
schedule = */5 * * * *
```

but somebody manually changes the generated Argo CronWorkflow:

```text
schedule = * * * * *
```

Because the controller watches the owned `CronWorkflow`:

```text
manual CronWorkflow update
          |
          v
Kubernetes event
          |
          v
controller-runtime queue
          |
          v
Reconcile()
          |
          v
restore */5 * * * *
```

The same mechanism can recreate a deleted child CronWorkflow.

---

## Status Synchronization

Desired configuration flows downward:

```text
MarketplaceWorkflow.spec
          |
          v
       Reconcile
          |
          v
CronWorkflow.spec
```

Observed state flows upward:

```text
CronWorkflow.status
          |
          v
      syncStatus()
          |
          v
MarketplaceWorkflow.status
```

Example:

```yaml
status:
  phase: Ready

  cronWorkflowName: publishing

  activeRuns: 1

  succeededRuns: 27

  failedRuns: 2

  observedGeneration: 4
```

If reconciliation cannot succeed, the controller reports the problem through Kubernetes conditions:

```yaml
status:
  phase: NotReady

  conditions:
    - type: Ready
      status: "False"
      reason: WorkflowTemplateNotFound
```

---

## Retry and Failure Handling

Worker processes classify failures using explicit exit codes.

| Exit Code | Failure Type | Retry |
|---|---|---|
| `0` | Success | No |
| `10` | Transient | Yes |
| `20` | Permanent | No |
| `30` | Validation | No |

Example Argo retry policy:

```yaml
retryStrategy:
  limit: "2"

  retryPolicy: OnFailure

  expression: "lastRetry.exitCode == '10'"

  backoff:
    duration: "5s"
    factor: 2
```

This keeps business failure classification inside Go while allowing Argo to manage retries and backoff.

---

## Project Structure

```text
marketplace-workflow-platform/

├── argo/
│   ├── templates/
│   │   └── publishing.yaml
│   │
│   └── cron/
│
├── worker/
│   ├── cmd/
│   │   └── worker/
│   │       └── main.go
│   │
│   ├── internal/
│   │   └── workflow/
│   │
│   ├── Dockerfile
│   └── go.mod
│
└── controller/
    ├── api/
    │   └── v1alpha1/
    │
    ├── internal/
    │   └── controller/
    │
    ├── config/
    │
    ├── cmd/
    │   └── main.go
    │
    ├── Makefile
    └── go.mod
```

---

## Technology Stack

- Go
- Kubernetes
- Kubebuilder
- controller-runtime
- Argo Workflows
- Docker
- Kind
- Custom Resource Definitions
- Kubernetes owner references
- envtest / Ginkgo / Gomega

---

## Local Development

### Requirements

Install:

```text
Docker Desktop
kubectl
kind
Go
Kubebuilder
Argo CLI
```

---

### Create the Kind Cluster

```bash
kind create cluster \
  --name marketplace-workflows
```

Verify:

```bash
kubectl cluster-info \
  --context kind-marketplace-workflows
```

---

### Install Argo Workflows

Install a compatible Argo Workflows release into the `argo` namespace.

Verify:

```bash
kubectl get pods -n argo
```

and:

```bash
kubectl get crd | grep argoproj
```

---

### Build the Marketplace Worker

```bash
cd worker

docker build \
  -t marketplace-worker:v0.2.0 .
```

Load it into Kind:

```bash
kind load docker-image \
  marketplace-worker:v0.2.0 \
  --name marketplace-workflows
```

---

### Install the Marketplace CRD

```bash
cd controller

make manifests
make install
```

Verify:

```bash
kubectl get crd | grep marketplaceworkflow
```

---

### Run Controller Locally

```bash
make run
```

The controller uses your current Kubernetes context.

---

### Install the Argo WorkflowTemplate

From the project root:

```bash
kubectl apply \
  -f argo/templates/publishing.yaml
```

Verify:

```bash
kubectl get workflowtemplates -n argo
```

---

### Create MarketplaceWorkflow

```bash
kubectl apply \
  -f controller/config/samples/publishing.yaml
```

Verify the custom resource:

```bash
kubectl get marketplaceworkflows -n argo
```

Then verify the controller-generated Argo CronWorkflow:

```bash
kubectl get cronworkflows -n argo
```

---

## Testing

The controller uses Kubernetes `envtest`.

Run:

```bash
cd controller

make test
```

The test environment starts a lightweight Kubernetes API server and etcd instance and installs the required CRDs.

The controller tests cover reconciliation behavior such as:

- resource creation
- desired-state updates
- missing WorkflowTemplates
- status synchronization
- owner references
- manual drift repair
- child recreation

---

## Kubernetes State

Kubernetes remains the source of truth for active orchestration state.

```text
Kubernetes API Server
        |
        v
       etcd

        |
        +-- MarketplaceWorkflow
        |
        +-- CronWorkflow
        |
        +-- Workflow
        |
        +-- Pod
```

The controller never talks directly to etcd.

It interacts with Kubernetes through the API server and controller-runtime client/cache.

Long-term workflow history or analytics can later be stored separately using Argo Workflow Archive or another external datastore.

---

## Design Principles

The project intentionally follows several Kubernetes controller principles:

- Declarative desired state
- Idempotent reconciliation
- Event-driven reconciliation
- Self-healing resources
- Clear resource ownership
- Separation of control plane and execution plane
- Kubernetes as the active state source of truth
- Argo as the workflow execution engine

---

## Current Status

Implemented:

- Custom `MarketplaceWorkflow` CRD
- Go/Kubebuilder controller
- Argo `CronWorkflow` reconciliation
- WorkflowTemplate validation
- Marketplace status synchronization
- Owner references
- Workflow retry classification
- Go worker containers
- Publishing DAG
- Local Kind development environment
- envtest controller testing

Planned production hardening:

- WorkflowTemplate event indexing/watch improvements
- Additional reconciliation predicates
- Kubernetes Events
- Prometheus controller metrics
- Configurable reconciliation concurrency
- Leader election / HA
- Controller Docker image
- Kubernetes Deployment and RBAC hardening
- End-to-end tests
- Argo workflow archival

---

## Goal

The goal of this project is to demonstrate how a domain-specific workflow platform can be built on Kubernetes without reimplementing a workflow engine.

The custom controller owns Marketplace-specific policy and desired state, while Argo Workflows owns scheduling and execution.

```text
Marketplace business policy
          |
          v
MarketplaceWorkflow
          |
          v
Custom Kubernetes Controller
          |
          v
Argo Workflows
          |
          v
Go Workers
```