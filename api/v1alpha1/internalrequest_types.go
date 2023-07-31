/*
Copyright 2022.

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
	"github.com/redhat-appstudio/operator-toolkit/conditions"
	"time"

	"github.com/redhat-appstudio/internal-services/metrics"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InternalRequestSpec defines the desired state of InternalRequest.
type InternalRequestSpec struct {
	// Request is the name of the internal internalrequest which will be translated into a Tekton pipeline
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Request string `json:"request"`

	// Params is the list of optional parameters to pass to the Tekton pipeline
	// kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Params map[string]string `json:"params,omitempty"`
}

// InternalRequestStatus defines the observed state of InternalRequest.
type InternalRequestStatus struct {
	// StartTime is the time when the InternalRequest PipelineRun was created and set to run
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the InternalRequest PipelineRun completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions represent the latest available observations for the internalrequest
	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// Results is the list of optional results as seen in the Tekton PipelineRun
	// kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Results map[string]string `json:"results,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Succeeded",type=string,JSONPath=`.status.conditions[?(@.type=="Succeeded")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Succeeded")].reason`

// InternalRequest is the Schema for the internalrequests API.
type InternalRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InternalRequestSpec   `json:"spec,omitempty"`
	Status InternalRequestStatus `json:"status,omitempty"`
}

// HasCompleted checks whether the InternalRequest has been completed.
func (ir *InternalRequest) HasCompleted() bool {
	condition := meta.FindStatusCondition(ir.Status.Conditions, SucceededConditionType.String())

	switch {
	case condition == nil:
		return false
	case condition.Status == metav1.ConditionTrue:
		return true
	default:
		return condition.Status == metav1.ConditionFalse && condition.Reason != RunningReason.String()
	}
}

// HasFailed checks whether the InternalRequest has failed.
func (ir *InternalRequest) HasFailed() bool {
	condition := meta.FindStatusCondition(ir.Status.Conditions, SucceededConditionType.String())

	switch {
	case condition == nil:
		return false
	case condition.Status == metav1.ConditionTrue:
		return false
	default:
		return condition.Status == metav1.ConditionFalse && condition.Reason != RunningReason.String()
	}
}

// HasSucceeded checks whether the InternalRequest has succeeded.
func (ir *InternalRequest) HasSucceeded() bool {
	return meta.IsStatusConditionTrue(ir.Status.Conditions, SucceededConditionType.String())
}

func (ir *InternalRequest) IsRunning() bool {
	condition := meta.FindStatusCondition(ir.Status.Conditions, SucceededConditionType.String())
	return condition != nil && condition.Status != metav1.ConditionTrue && condition.Reason == RunningReason.String()
}

// MarkFailed registers the completion time and changes the Succeeded condition to False with the provided message.
func (ir *InternalRequest) MarkFailed(message string) {
	if ir.HasCompleted() {
		return
	}

	ir.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	conditions.SetConditionWithMessage(&ir.Status.Conditions, SucceededConditionType, metav1.ConditionFalse, FailedReason, message)

	go metrics.RegisterCompletedInternalRequest(ir.Spec.Request, ir.Namespace, FailedReason.String(),
		ir.Status.StartTime, ir.Status.CompletionTime, false)
}

// MarkRejected changes the Succeeded condition to False with the provided reason and message.
func (ir *InternalRequest) MarkRejected(message string) {
	if ir.HasCompleted() {
		return
	}

	conditions.SetConditionWithMessage(&ir.Status.Conditions, SucceededConditionType, metav1.ConditionFalse, RejectedReason, message)

}

// MarkRunning registers the start time and changes the Succeeded condition to Unknown.
func (ir *InternalRequest) MarkRunning() {
	if ir.HasCompleted() {
		return
	}

	if !ir.IsRunning() {
		ir.Status.StartTime = &metav1.Time{Time: time.Now()}
	}

	conditions.SetCondition(&ir.Status.Conditions, SucceededConditionType, metav1.ConditionFalse, RunningReason)
}

// MarkSucceeded registers the completion time and changes the Succeeded condition to True.
func (ir *InternalRequest) MarkSucceeded() {
	if ir.HasCompleted() {
		return
	}

	ir.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	conditions.SetCondition(&ir.Status.Conditions, SucceededConditionType, metav1.ConditionTrue, SucceededReason)

	go metrics.RegisterCompletedInternalRequest(ir.Spec.Request, ir.Namespace, SucceededReason.String(), ir.Status.StartTime, ir.Status.CompletionTime, true)
}

// +kubebuilder:object:root=true

// InternalRequestList contains a list of InternalRequest.
type InternalRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternalRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InternalRequest{}, &InternalRequestList{})
}
