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

package scope

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/version"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new Scope.
type ManagedControlPlaneScopeParams struct {
	AWSClients
	Client            client.Client
	Logger            logr.Logger
	Cluster           *clusterv1.Cluster
	AWSManagedCluster *infrav1exp.AWSManagedCluster
	ControlPlane      *infrav1exp.AWSManagedControlPlane
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.AWSManagedCluster == nil {
		return nil, errors.New("failed to generate new scope from nil AWSManagedCluster")
	}
	if params.ControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil EKSControlPlane")
	}
	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	session, err := sessionForRegion(params.AWSManagedCluster.Spec.Region)
	if err != nil {
		return nil, errors.Errorf("failed to create aws session: %v", err)
	}

	userAgentHandler := request.NamedHandler{
		Name: "capa/user-agent",
		Fn:   request.MakeAddToUserAgentHandler("eks.control-plane.cluster.x-k8s.io", version.Get().String()),
	}

	if params.AWSClients.EKS == nil {
		eksClient := eks.New(session)
		eksClient.Handlers.Build.PushFrontNamed(userAgentHandler)
		eksClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.ControlPlane))
		params.AWSClients.EKS = eksClient
	}
	if params.AWSClients.IAM == nil {
		iamClient := iam.New(session)
		iamClient.Handlers.Build.PushFrontNamed(userAgentHandler)
		iamClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.ControlPlane))
		params.AWSClients.IAM = iamClient
	}

	if params.AWSClients.ResourceTagging == nil {
		resourceTagging := resourcegroupstaggingapi.New(session)
		resourceTagging.Handlers.Build.PushFrontNamed(userAgentHandler)
		resourceTagging.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.ControlPlane))
		params.AWSClients.ResourceTagging = resourceTagging
	}

	helper, err := patch.NewHelper(params.ControlPlane, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedControlPlaneScope{
		Logger:            params.Logger,
		Client:            params.Client,
		AWSClients:        params.AWSClients,
		Cluster:           params.Cluster,
		AWSManagedCluster: params.AWSManagedCluster,
		ControlPlane:      params.ControlPlane,
		patchHelper:       helper,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper

	AWSClients
	Cluster           *clusterv1.Cluster
	AWSManagedCluster *infrav1exp.AWSManagedCluster
	ControlPlane      *infrav1exp.AWSManagedControlPlane
}

// Network returns the control plane network object.
func (s *ManagedControlPlaneScope) Network() *infrav1.Network {
	return &s.AWSManagedCluster.Status.Network
}

// VPC returns the control plane VPC.
func (s *ManagedControlPlaneScope) VPC() *infrav1.VPCSpec {
	return &s.AWSManagedCluster.Spec.NetworkSpec.VPC
}

// Subnets returns the control plane subnets.
func (s *ManagedControlPlaneScope) Subnets() infrav1.Subnets {
	return s.AWSManagedCluster.Spec.NetworkSpec.Subnets
}

// SecurityGroups returns the control plane security groups as a map, it creates the map if empty.
func (s *ManagedControlPlaneScope) SecurityGroups() map[infrav1.SecurityGroupRole]infrav1.SecurityGroup {
	return s.AWSManagedCluster.Status.Network.SecurityGroups
}

// Name returns the cluster name.
func (s *ManagedControlPlaneScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ManagedControlPlaneScope) Namespace() string {
	return s.Cluster.Namespace
}

// Region returns the cluster region.
func (s *ManagedControlPlaneScope) Region() string {
	return s.AWSManagedCluster.Spec.Region
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ManagedControlPlaneScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the control plane configuration and status.
func (s *ManagedControlPlaneScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.ControlPlane)
}

// Close closes the current scope persisting the control plane configuration and status.
func (s *ManagedControlPlaneScope) Close() error {
	return s.PatchObject()
}

// AdditionalTags returns AdditionalTags from the scope's EksControlPlane. The returned value will never be nil.
func (s *ManagedControlPlaneScope) AdditionalTags() infrav1.Tags {
	if s.AWSManagedCluster.Spec.AdditionalTags == nil {
		s.AWSManagedCluster.Spec.AdditionalTags = infrav1.Tags{}
	}

	return s.AWSManagedCluster.Spec.AdditionalTags.DeepCopy()
}
