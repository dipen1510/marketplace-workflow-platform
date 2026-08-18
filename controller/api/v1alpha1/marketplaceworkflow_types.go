/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ConcurrencyPolicy string

const (
	ConcurrencyAllow   ConcurrencyPolicy = "Allow"
	ConcurrencyForbid  ConcurrencyPolicy = "Forbid"
	ConcurrencyReplace ConcurrencyPolicy = "Replace"
)

type WorkflowTemplateReference struct {
	Name string `json:"name"`
}

type WorkflowParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MarketplaceWorkflowSpec defines the desired state of MarketplaceWorkflow
type MarketplaceWorkflowSpec struct {

	// Schedules defines when this workflow runs.
	// +kubebuilder:validation:MinItems=1
	Schedules []string `json:"schedules"`

	// Timezone used when evaluating the schedules.
	// +optional
	Timezone string `json:"timezone,omitempty"`

	// Suspend prevents new scheduled executions.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	// +kubebuilder:default=Forbid
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// Argo WorkflowTemplate that will actually execute.
	WorkflowTemplateRef WorkflowTemplateReference `json:"workflowTemplateRef"`

	// Parameters passed to the Argo workflow.
	// +optional
	Parameters []WorkflowParameter `json:"parameters,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	SuccessfulRunsHistoryLimit *int32 `json:"successfulRunsHistoryLimit,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +optional
	FailedRunsHistoryLimit *int32 `json:"failedRunsHistoryLimit,omitempty"`
}

type MarketplaceWorkflowPhase string

const (
	MarketplaceWorkflowReady    MarketplaceWorkflowPhase = "Ready"
	MarketplaceWorkflowNotReady MarketplaceWorkflowPhase = "NotReady"
)

// MarketplaceWorkflowStatus defines the observed state of MarketplaceWorkflow.
type MarketplaceWorkflowStatus struct {

	// Current high-level state.
	// +optional
	Phase MarketplaceWorkflowPhase `json:"phase,omitempty"`

	// Name of the Argo CronWorkflow created by this controller.
	// +optional
	CronWorkflowName string `json:"cronWorkflowName,omitempty"`

	// Current number of active Argo runs.
	// +optional
	ActiveRuns int32 `json:"activeRuns,omitempty"`

	// Last time Argo scheduled this workflow.
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

	// Number of successful executions reported by Argo.
	// +optional
	SucceededRuns int64 `json:"succeededRuns,omitempty"`

	// Number of failed executions reported by Argo.
	// +optional
	FailedRuns int64 `json:"failedRuns,omitempty"`

	// Generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Standard Kubernetes conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MarketplaceWorkflow is the Schema for the marketplaceworkflows API
type MarketplaceWorkflow struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of MarketplaceWorkflow
	// +required
	Spec MarketplaceWorkflowSpec `json:"spec"`

	// status defines the observed state of MarketplaceWorkflow
	// +optional
	Status MarketplaceWorkflowStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MarketplaceWorkflowList contains a list of MarketplaceWorkflow
type MarketplaceWorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []MarketplaceWorkflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &MarketplaceWorkflow{}, &MarketplaceWorkflowList{})
		return nil
	})
}
