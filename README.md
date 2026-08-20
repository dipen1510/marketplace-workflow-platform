# Marketplace Workflow Platform

A production-style local Kubernetes workflow platform for Marketplace publishing workflows, built with **Go**, **Argo Workflows**, **client-go**, **Prometheus**, **Grafana**, and **Kind**.

The project demonstrates:

- Scheduled workflows with Argo `CronWorkflow`
- DAG-based workflow execution with retries and failure classification
- Go worker containers executed by Argo
- A low-level `client-go` status controller using `SharedInformer` + rate-limiting workqueue
- REST-based workflow status synchronization
- Kubernetes RBAC and in-cluster authentication
- Prometheus metrics from both Argo and the custom status controller
- Prometheus persistent storage with a Kubernetes PVC
- Local end-to-end deployment on a Kind cluster

> This repository is intentionally a learning / architecture project. The Publishing service is a local mock implementation that keeps workflow status in memory, while the worker simulates calls that would exist in a production Marketplace platform.

---

## Architecture

### Execution plane

```text
                           Kubernetes / Kind

                         Argo CronWorkflow
                                |
                                | schedule
                                v
                           Argo Workflow
                                |
                    +-----------+-----------+
                    |           |           |
                    v           v           v
                 discover    validate     publish
                    |           |           |
                    +-----------+-----------+
                                |
                                v
                     marketplace-worker
                           Go container
                                |
                                v
                 Marketplace / OCI service calls
                      simulated in this repo
```

Argo owns scheduling, DAG execution, retries, step dependencies, workflow state, and workflow execution pods.

The Publishing workflow is:

```text
discover
   |
   v
validate
   |
   v
publish
```

The `validate` and `publish` templates use Argo retry policies. Exit code `10` represents a transient error and is retryable.

---

### Status plane

```text
                     Argo Workflow.status
                              |
                              | Kubernetes LIST / WATCH
                              v
                      SharedInformerFactory
                              |
                              v
                         Informer Cache
                              |
                    ADD / UPDATE / DELETE
                              |
                              v
                Typed Rate-Limiting WorkQueue
                              |
                              v
                       Worker Goroutines
                              |
                              v
                     Workflow Status Mapper
                              |
                              | HTTP PUT
                              v
                  Marketplace Publishing Service
                              |
                              v
                       In-memory Job Store
```

The status controller watches Argo Workflows in the `argo` namespace.

It does **not** poll Argo.

It uses Kubernetes-native:

- `SharedInformer`
- informer cache
- typed rate-limiting workqueue
- worker goroutines
- exponential retry
- Kubernetes ServiceAccount + RBAC
- `rest.InClusterConfig()` when deployed inside Kubernetes

The REST status API used by the controller is:

```text
PUT /v1/workflows/{workflowUID}/status
```

The local Publishing service stores the latest workflow snapshot in memory.

---

### Observability plane

```text
                         Prometheus
                         /        \
                        /          \
                       v            v

            Argo Workflow       Status Controller
              Controller            /metrics
               /metrics
                   \                  /
                    \                /
                     +--------------+
                            |
                            v
                        Prometheus
                           TSDB
                            |
                            v
                         200Mi PVC
                            |
                            v
                         Grafana
```

Metrics have separate ownership:

**Argo**

- Workflow result
- Workflow duration
- Step result
- Step duration
- Retry count

**Marketplace Status Controller**

- Informer events
- Workqueue depth
- Status synchronization success/failure
- Synchronization retries
- Dropped status updates
- Publishing REST response classes
- Publishing REST latency

---

## Repository structure

```text
marketplace-workflow-platform/
|
├── argo/
|   ├── templates/
|   |   ├── publishing.yaml
|   |   └── image-validation.yaml
|   |
|   └── cron/
|       ├── publishing-cron.yaml
|       └── image-validation-cron.yaml
|
├── worker/
|   ├── Dockerfile
|   ├── go.mod
|   ├── cmd/
|   |   └── worker/
|   |       └── main.go
|   └── internal/
|       └── workflow/
|
├── publishing-status-controller/
|   ├── Dockerfile
|   ├── Dockerfile.publishing-service
|   ├── go.mod
|   ├── cmd/
|   |   ├── controller/
|   |   └── publishing-service/
|   ├── internal/
|   |   ├── controller/
|   |   ├── metrics/
|   |   ├── model/
|   |   └── publishing/
|   └── deploy/
|       ├── namespace.yaml
|       ├── rbac.yaml
|       ├── publishing-service.yaml
|       ├── controller.yaml
|       └── service.yaml
|
└── monitoring/
    ├── prometheus-values.yaml
    ├── argo-metrics-service.yaml
    ├── argo-service-monitor.yaml
    └── status-controller-service-monitor.yaml
```

---

# Local prerequisites

Recommended local environment:

- Docker Desktop
- Kind
- kubectl
- Helm
- Argo CLI
- Git

Optional for local Go development:

- Go `1.26.5`

Verify:

```bash
docker version
kind version
kubectl version --client
helm version
argo version
git --version
```

The examples below use:

```text
Kind cluster: marketplace-workflows
Argo namespace: argo
Application namespace: marketplace-system
Monitoring namespace: monitoring
```

---

# Quick start: zero to running platform

Run all commands from the repository root:

```text
marketplace-workflow-platform/
```

## 1. Clone the repository

```bash
git clone https://github.com/dipen1510/marketplace-workflow-platform.git
cd marketplace-workflow-platform
```

---

## 2. Create the Kind cluster

```bash
kind create cluster --name marketplace-workflows
```

Verify:

```bash
kind get clusters
kubectl cluster-info --context kind-marketplace-workflows
kubectl get nodes
```

Expected:

```text
marketplace-workflows-control-plane   Ready
```

---

## 3. Install Argo Workflows

Create the namespace:

```bash
kubectl create namespace argo
```

Install Argo Workflows v4.0.8:

```bash
kubectl apply \
  -n argo \
  -f https://github.com/argoproj/argo-workflows/releases/download/v4.0.8/install.yaml
```

Wait for the controller:

```bash
kubectl rollout status \
  deployment/workflow-controller \
  -n argo
```

Check Argo:

```bash
kubectl get pods -n argo
```

You should see the Argo workflow controller running.

Depending on the Argo install manifest or other local examples you have applied, you may also see components such as `argo-server`, MinIO, or other demo services.

---

## 4. Build the Marketplace worker image

The Argo templates use:

```text
marketplace-worker:v0.2.0
```

Build:

```bash
docker build \
  -t marketplace-worker:v0.2.0 \
  -f worker/Dockerfile \
  worker
```

Load the image into Kind:

```bash
kind load docker-image \
  marketplace-worker:v0.2.0 \
  --name marketplace-workflows
```

---

## 5. Build the status controller image

```bash
docker build \
  -t marketplace-status-controller:v0.1.0 \
  -f publishing-status-controller/Dockerfile \
  publishing-status-controller
```

Load it:

```bash
kind load docker-image \
  marketplace-status-controller:v0.1.0 \
  --name marketplace-workflows
```

---

## 6. Build the local Publishing service image

```bash
docker build \
  -t marketplace-publishing-service:v0.1.0 \
  -f publishing-status-controller/Dockerfile.publishing-service \
  publishing-status-controller
```

Load it:

```bash
kind load docker-image \
  marketplace-publishing-service:v0.1.0 \
  --name marketplace-workflows
```

Verify the images exist locally:

```bash
docker images | grep marketplace
```

Expected image tags:

```text
marketplace-worker               v0.2.0
marketplace-status-controller    v0.1.0
marketplace-publishing-service   v0.1.0
```

---

## 7. Deploy the application namespace and RBAC

Create the application namespace:

```bash
kubectl apply \
  -f publishing-status-controller/deploy/namespace.yaml
```

Apply RBAC:

```bash
kubectl apply \
  -f publishing-status-controller/deploy/rbac.yaml
```

The status controller receives only:

```text
get
list
watch
```

for Argo `Workflow` resources in namespace `argo`.

Verify:

```bash
kubectl auth can-i \
  list workflows.argoproj.io \
  -n argo \
  --as=system:serviceaccount:marketplace-system:marketplace-status-controller
```

Expected:

```text
yes
```

Check that it cannot delete workflows:

```bash
kubectl auth can-i \
  delete workflows.argoproj.io \
  -n argo \
  --as=system:serviceaccount:marketplace-system:marketplace-status-controller
```

Expected:

```text
no
```

---

## 8. Deploy the local Publishing service

```bash
kubectl apply \
  -f publishing-status-controller/deploy/publishing-service.yaml
```

Wait:

```bash
kubectl rollout status \
  deployment/marketplace-publishing-service \
  -n marketplace-system
```

The Kubernetes service DNS name is:

```text
marketplace-publishing-service.marketplace-system.svc.cluster.local:8080
```

---

## 9. Deploy the Marketplace status controller

```bash
kubectl apply \
  -f publishing-status-controller/deploy/controller.yaml

kubectl apply \
  -f publishing-status-controller/deploy/service.yaml
```

Wait:

```bash
kubectl rollout status \
  deployment/marketplace-status-controller \
  -n marketplace-system
```

Verify:

```bash
kubectl get pods -n marketplace-system
kubectl get svc -n marketplace-system
```

Expected:

```text
marketplace-publishing-service-...   1/1 Running
marketplace-status-controller-...    1/1 Running
```

The status controller uses:

```text
ServiceAccount
     |
     v
rest.InClusterConfig()
     |
     v
Kubernetes API
     |
     v
LIST/WATCH Argo Workflows
```

No local kubeconfig is required inside the controller pod.

---

# Install monitoring

## 10. Install kube-prometheus-stack

Add the Helm repository:

```bash
helm repo add prometheus-community \
  https://prometheus-community.github.io/helm-charts

helm repo update
```

Install Prometheus Operator, Prometheus, Grafana, Alertmanager, and related components:

```bash
helm upgrade --install kube-prometheus-stack \
  prometheus-community/kube-prometheus-stack \
  -n monitoring \
  --create-namespace \
  -f monitoring/prometheus-values.yaml
```

Check:

```bash
kubectl get pods -n monitoring
```

Verify the ServiceMonitor CRD exists:

```bash
kubectl get crd servicemonitors.monitoring.coreos.com
```

---

## Prometheus local storage

`monitoring/prometheus-values.yaml` configures:

```text
Time retention:      2 days
Size retention:      150MB
PVC capacity:        200Mi
Access mode:         ReadWriteOnce
```

Verify the PVC:

```bash
kubectl get pvc -n monitoring
```

Expected:

```text
STATUS   CAPACITY   ACCESS MODES
Bound    200Mi      RWO
```

The local flow is:

```text
Prometheus Pod
      |
      v
Prometheus TSDB
      |
      v
PVC
      |
      v
Kind local-path PersistentVolume
```

The PVC allows Prometheus history to survive a Prometheus pod recreation.

It does **not** survive deleting the entire Kind cluster.

---

## 11. Expose Argo controller metrics to Prometheus

The default Argo installation exposes workflow-controller metrics from the pod, but this project creates a Service so Prometheus can discover them.

Apply:

```bash
kubectl apply \
  -f monitoring/argo-metrics-service.yaml
```

Verify:

```bash
kubectl get svc \
  -n argo \
  workflow-controller-metrics \
  --show-labels
```

Check the EndpointSlice:

```bash
kubectl get endpointslice \
  -n argo \
  -l kubernetes.io/service-name=workflow-controller-metrics \
  -o wide
```

---

## 12. Create the Argo ServiceMonitor

```bash
kubectl apply \
  -f monitoring/argo-service-monitor.yaml
```

This connects:

```text
Prometheus
    |
    v
ServiceMonitor
    |
    v
workflow-controller-metrics Service
    |
    v
Argo workflow-controller :9090/metrics
```

---

## 13. Create the status-controller ServiceMonitor

```bash
kubectl apply \
  -f monitoring/status-controller-service-monitor.yaml
```

This connects:

```text
Prometheus
    |
    v
ServiceMonitor
    |
    v
marketplace-status-controller Service
    |
    v
Status Controller :9090/metrics
```

Verify both monitors:

```bash
kubectl get servicemonitor -n monitoring
```

---

# Install workflows

## 14. Apply WorkflowTemplates

Optional lint step:

```bash
argo template lint argo/templates/publishing.yaml
argo template lint argo/templates/image-validation.yaml
```

Apply:

```bash
kubectl apply \
  -f argo/templates/publishing.yaml

kubectl apply \
  -f argo/templates/image-validation.yaml
```

Verify:

```bash
kubectl get workflowtemplates -n argo
```

Expected:

```text
marketplace-publishing
marketplace-image-validation
```

---

# Run the application

## 15. Run the Publishing workflow manually

Start with a successful workflow before enabling scheduled runs:

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=success \
  --watch
```

Argo creates worker pods automatically.

You do **not** manually start the worker container.

The lifecycle is:

```text
argo submit
     |
     v
Workflow
     |
     v
discover worker Pod
     |
     v
validate worker Pod
     |
     v
publish worker Pod
     |
     v
Workflow Succeeded
```

Check workflows:

```bash
argo list -n argo
```

Check pods:

```bash
kubectl get pods -n argo
```

---

## 16. Watch status-controller activity

In another terminal:

```bash
kubectl logs \
  -n marketplace-system \
  deployment/marketplace-status-controller \
  -f
```

Typical events:

```text
[EVENT ADD]
[EVENT UPDATE]
[QUEUE ADD]
[WORKER ...] processing
[SYNC JOB]
[REST] synchronized workflow=...
```

---

## 17. Watch the Publishing service

```bash
kubectl logs \
  -n marketplace-system \
  deployment/marketplace-publishing-service \
  -f
```

Typical output:

```text
[PUBLISHING] CREATE workflow=...
[PUBLISHING] UPDATE workflow=...
    step=discover ...
    step=validate ...
    step=publish ...
```

The local service stores only the latest workflow snapshot in memory.

---

# Enable scheduled workflows

## 18. Apply CronWorkflows

Publishing:

```bash
kubectl apply \
  -f argo/cron/publishing-cron.yaml
```

Image validation:

```bash
kubectl apply \
  -f argo/cron/image-validation-cron.yaml
```

Current development schedules:

```text
Publishing:        every 2 minutes
Image validation:  every 5 minutes
```

Both use:

```text
concurrencyPolicy: Forbid
```

so a new scheduled execution is not started while the previous one is still running.

Check:

```bash
argo cron list -n argo
argo list -n argo
```

---

# Prometheus

## 19. Open Prometheus locally

Port-forward:

```bash
kubectl port-forward \
  -n monitoring \
  svc/kube-prometheus-stack-prometheus \
  9092:9090
```

Open:

```text
http://localhost:9092
```

Check:

```text
Status -> Target health
```

Expected targets include:

```text
argo-workflow-controller       UP
marketplace-status-controller  UP
```

---

## Argo custom metrics

The Publishing WorkflowTemplate defines custom Argo metrics.

Argo exposes custom metrics with the `argo_workflows_` prefix.

Useful queries:

```promql
argo_workflows_marketplace_workflow_result_total
```

```promql
argo_workflows_marketplace_workflow_step_result_total
```

```promql
argo_workflows_marketplace_workflow_step_retry_total
```

```promql
argo_workflows_marketplace_workflow_duration_seconds_count
```

```promql
argo_workflows_marketplace_workflow_step_duration_seconds_count
```

P95 workflow duration:

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(
      argo_workflows_marketplace_workflow_duration_seconds_bucket[10m]
    )
  )
)
```

---

## Status-controller metrics

Informer events:

```promql
marketplace_status_controller_events_total
```

Queue depth:

```promql
marketplace_status_controller_queue_depth
```

Status synchronization attempts:

```promql
marketplace_status_controller_sync_total
```

Synchronization retries:

```promql
marketplace_status_controller_sync_retries_total
```

Dropped status updates:

```promql
marketplace_status_controller_sync_dropped_total
```

REST requests:

```promql
marketplace_status_controller_http_requests_total
```

REST latency:

```promql
marketplace_status_controller_http_request_duration_seconds_count
```

P95 Publishing REST latency:

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(
      marketplace_status_controller_http_request_duration_seconds_bucket[10m]
    )
  )
)
```

---

## Counter resets

Prometheus counters are stored in process memory.

For example:

```text
marketplace_status_controller_sync_retries_total
```

resets to zero if the status-controller process restarts.

Prometheus retains the previously scraped history in its TSDB, so use PromQL functions such as:

```promql
increase(
  marketplace_status_controller_sync_retries_total[1h]
)
```

or:

```promql
rate(
  marketplace_status_controller_sync_retries_total[5m]
)
```

instead of manually subtracting raw counter values.

---

# Grafana

## 20. Open Grafana

Get the admin password:

```bash
kubectl get secret \
  -n monitoring \
  kube-prometheus-stack-grafana \
  -o jsonpath="{.data.admin-password}" \
  | base64 -d

echo
```

Port-forward:

```bash
kubectl port-forward \
  -n monitoring \
  svc/kube-prometheus-stack-grafana \
  3000:80
```

Open:

```text
http://localhost:3000
```

Login:

```text
username: admin
password: <value from the secret>
```

Prometheus is provisioned as a Grafana datasource by kube-prometheus-stack.

---

# Failure testing

The worker uses exit codes to classify errors:

```text
0   success
10  transient / retryable
20  permanent
30  validation failure
```

Argo retries only exit code `10` for templates that define the retry strategy.

## Successful validation

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=success \
  --watch
```

## Transient validation failure

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=transient \
  --watch
```

The validation worker fails transiently and later succeeds after retry.

Then query:

```promql
argo_workflows_marketplace_workflow_step_retry_total
```

## Permanent validation failure

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=permanent \
  --watch
```

## Invalid image

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=invalid-image \
  --watch
```

---

# Test status-controller retry behavior

Temporarily stop the Publishing service:

```bash
kubectl scale \
  deployment/marketplace-publishing-service \
  -n marketplace-system \
  --replicas=0
```

Run a workflow:

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=success \
  --watch
```

Watch the controller:

```bash
kubectl logs \
  -n marketplace-system \
  deployment/marketplace-status-controller \
  -f
```

Useful PromQL:

```promql
marketplace_status_controller_sync_total{result="failure"}
```

```promql
marketplace_status_controller_sync_retries_total
```

Restore the service:

```bash
kubectl scale \
  deployment/marketplace-publishing-service \
  -n marketplace-system \
  --replicas=1
```

This demonstrates an important distributed-systems condition:

```text
Workflow execution can be healthy
while
status propagation is temporarily unhealthy
```

---

# Verify Prometheus persistence

First verify the PVC:

```bash
kubectl get pvc -n monitoring
```

Then delete the Prometheus pod:

```bash
kubectl delete pod \
  -n monitoring \
  prometheus-kube-prometheus-stack-prometheus-0
```

The StatefulSet recreates it.

Watch:

```bash
kubectl get pods -n monitoring -w
```

Verify that the same claim is still bound:

```bash
kubectl get pvc -n monitoring
```

Previously scraped Prometheus data should remain available after the pod is recreated.

---

# Workflow pod retention

Long-running infrastructure pods such as:

```text
workflow-controller
argo-server
Prometheus
marketplace-status-controller
marketplace-publishing-service
```

are Deployment/StatefulSet workloads and are expected to remain running.

Argo execution pods such as:

```text
marketplace-publishing-...-discover-...
marketplace-publishing-...-validate-...
marketplace-publishing-...-publish-...
```

are short-lived workflow execution pods.

The current repository does not configure an 8-hour PodGC policy in the Publishing WorkflowTemplate.

If you want completed Argo execution pods to remain available for debugging for up to eight hours, add this to the top-level `spec` in `argo/templates/publishing.yaml`:

```yaml
podGC:
  strategy: OnPodCompletion
  deleteDelayDuration: 8h
```

Then reapply:

```bash
kubectl apply \
  -f argo/templates/publishing.yaml
```

---

# Troubleshooting

## ImagePullBackOff / ErrImageNeverPull

The images are local Docker images and must be loaded into the Kind node:

```bash
kind load docker-image \
  marketplace-worker:v0.2.0 \
  --name marketplace-workflows

kind load docker-image \
  marketplace-status-controller:v0.1.0 \
  --name marketplace-workflows

kind load docker-image \
  marketplace-publishing-service:v0.1.0 \
  --name marketplace-workflows
```

---

## Status controller cannot list Workflows

Check RBAC:

```bash
kubectl auth can-i \
  list workflows.argoproj.io \
  -n argo \
  --as=system:serviceaccount:marketplace-system:marketplace-status-controller
```

Expected:

```text
yes
```

Check controller logs:

```bash
kubectl logs \
  -n marketplace-system \
  deployment/marketplace-status-controller
```

---

## Prometheus shows `0 / 0` Argo targets

Verify the Argo metrics Service exists:

```bash
kubectl get svc \
  -n argo \
  workflow-controller-metrics \
  --show-labels
```

It must include:

```text
monitoring=marketplace
```

Verify endpoints:

```bash
kubectl get endpointslice \
  -n argo \
  -l kubernetes.io/service-name=workflow-controller-metrics \
  -o wide
```

Verify the ServiceMonitor:

```bash
kubectl get servicemonitor \
  -n monitoring \
  argo-workflow-controller \
  -o yaml
```

---

## Prometheus PVC is missing

Check:

```bash
kubectl get prometheus -n monitoring
kubectl get storageclass
kubectl get pvc -n monitoring
```

The Kind cluster normally provides:

```text
standard
rancher.io/local-path
```

Check the Prometheus Operator if reconciliation is not happening:

```bash
kubectl logs \
  -n monitoring \
  deployment/kube-prometheus-stack-operator \
  --tail=100
```

---

## See why a pod is not starting

```bash
kubectl describe pod \
  -n <namespace> \
  <pod-name>
```

Look at the `Events` section.

---

# Development notes and current limitations

This repository intentionally simplifies several production concerns.

### Publishing service

Current:

```text
HTTP service
    |
    v
in-memory map[workflowUID]JobStatus
```

A production implementation would persist workflow state in a durable database such as OCI NoSQL, PostgreSQL, or another appropriate store.

### Worker operations

The Go worker currently simulates Marketplace and OCI service interactions.

A production implementation would call actual Publishing, Object Storage, image validation, Publisher, and other service APIs.

### Image-validation metrics

The Publishing WorkflowTemplate currently contains the custom Argo Prometheus metrics.

The Image Validation template can be instrumented in the same way if independent workflow metrics are required.

### Long-term Prometheus storage

The local environment uses:

```text
Prometheus -> 200Mi PVC
```

For production, do not periodically copy the live Prometheus PVC into Object Storage.

A typical long-term architecture is:

```text
Prometheus
    |
    | short-term TSDB
    v
PVC
    |
    v
Thanos Sidecar
    |
    v
OCI Object Storage
    |
    v
Thanos Store / Query
    |
    v
Grafana
```

This allows Grafana to query months of metrics through a Prometheus-compatible query layer while Prometheus keeps only shorter local retention.

---

# Optional local Go tests

Both Go modules currently target Go `1.26.5`.

Worker:

```bash
cd worker
go test ./...
cd ..
```

Status controller:

```bash
cd publishing-status-controller
go test ./...
cd ..
```

Docker builds do not require Go to be installed locally because the Dockerfiles compile the binaries in Go builder images.

---

# Useful inspection commands

Cluster:

```bash
kubectl get nodes
kubectl get pods -A
```

Argo:

```bash
argo list -n argo
argo cron list -n argo
kubectl get workflowtemplates -n argo
kubectl get cronworkflows -n argo
```

Application:

```bash
kubectl get all -n marketplace-system
```

Monitoring:

```bash
kubectl get pods -n monitoring
kubectl get servicemonitor -n monitoring
kubectl get pvc -n monitoring
```

Status-controller logs:

```bash
kubectl logs \
  -n marketplace-system \
  deployment/marketplace-status-controller \
  -f
```

Publishing-service logs:

```bash
kubectl logs \
  -n marketplace-system \
  deployment/marketplace-publishing-service \
  -f
```

---

# Cleanup

Delete the entire local environment:

```bash
kind delete cluster \
  --name marketplace-workflows
```

Because the Prometheus PVC is backed by Kind local-path storage, deleting the Kind cluster also deletes the local Prometheus history.

---

# End-to-end flow summary

```text
                         SCHEDULING

                    Argo CronWorkflow
                           |
                           v

                         EXECUTION

                      Argo Workflow
                           |
            +--------------+--------------+
            |              |              |
            v              v              v
         discover       validate       publish
            |              |              |
            +--------------+--------------+
                           |
                           v
                    Go Worker Pods


                       STATUS SYNC

                    Workflow.status
                           |
                           v
                    SharedInformer
                           |
                           v
                     WorkQueue
                           |
                           v
                    Status Workers
                           |
                           | REST
                           v
                 Publishing Status API


                      OBSERVABILITY

             Argo Controller     Status Controller
                    \                /
                     \              /
                      v            v
                        Prometheus
                           |
                           v
                         200Mi PVC
                           |
                           v
                         Grafana
```

The core design principle is separation of concerns:

- **Argo** owns scheduling and workflow execution.
- **Go workers** own individual workflow operations.
- **The status controller** observes Argo and synchronizes Marketplace-facing workflow status.
- **Prometheus** owns operational time-series history.
- **Grafana** provides visualization and dashboards.
