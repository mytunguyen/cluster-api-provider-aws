/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
)

const (
	// EksControlPlaneFinalizer allows teh controller to cldean up resources on delete
	EksControlPlaneFinalizer = "eks.controlplane.cluster.x-k8s.io"
)

// EksControlPlaneSpec defines the desired state of EksControlPlane
type EksControlPlaneSpec struct {
	// Version defines the desired Kubernetes version.
	// +kubebuilder:validation:MinLength:=2
	// +kubebuilder:validation:Pattern:=^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$
	Version string `json:"version"`

	// RoleArn specifies the ARN of the IAM role that gives EKS
	// permission to make API calls
	// +kubebuilder:validation:MinLength:=2
	RoleArn string `json:"roleArn"`

	// EncryptionConfig specifies the encryption configuration for the cluster
	// +optional
	EncryptionConfig *[]EncryptionConfig `json:"encryptionConfig,omitempty"`

	// AdditionalTags is an optional set of tags to add to AWS resources managed by the AWS provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`

	// Private indicates if the control plane should be private
	// +optional
	Private *bool `json:"private,omitempty"`
}

// EncryptionConfig specifies the encryption configuration for the EKS clsuter
type EncryptionConfig struct {
	// Provider specifies the ARN or alias of the CMK (in AWS KMS)
	Provider *infrav1.AWSResourceReference `json:"provider,omitempty"`
	//Resources specifies the resources to be encrypted
	Resources []*string `json:"resources,omitempty"`
}

// EksControlPlaneStatus defines the observed state of EksControlPlane
type EksControlPlaneStatus struct {
	// Initialized denotes whether or not the control plane has the
	// uploaded kubeadm-config configmap.
	// +kubebuilder:default=false
	Initialized bool `json:"initialized"`

	// Ready denotes that the KubeadmControlPlane API Server is ready to
	// receive requests.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// FailureReason indicates that there is a terminal problem reconciling the
	// state, and will be set to a token value suitable for
	// programmatic interpretation.
	// +optional
	//FailureReason errors.KubeadmControlPlaneStatusError `json:"failureReason,omitempty"`

	// ErrorMessage indicates that there is a terminal problem reconciling the
	// state, and will be set to a descriptive error message.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ekscontrolplanes,shortName=ecp,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=".status.ready",description="EKSControlPlane API Server is ready to receive requests"
// +kubebuilder:printcolumn:name="Initialized",type=boolean,JSONPath=".status.initialized",description="This denotes whether or not the control plane has the uploaded kubeadm-config configmap"

// EksControlPlane is the Schema for the ekscontrolplanes API
type EksControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EksControlPlaneSpec   `json:"spec,omitempty"`
	Status EksControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EksControlPlaneList contains a list of EksControlPlane
type EksControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EksControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EksControlPlane{}, &EksControlPlaneList{})
}
