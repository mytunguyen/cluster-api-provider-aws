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
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api-provider-aws/controlplane/eks/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/version"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new Scope.
type ManagedControlPlaneScopeParams struct {
	AWSClients
	Client          client.Client
	Logger          logr.Logger
	Cluster         *clusterv1.Cluster
	EksControlPlane *controlplanev1.EksControlPlane
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.EksControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil EksControlPlane")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	session, err := sessionForRegion(params.EksControlPlane.Spec.Region)
	if err != nil {
		return nil, errors.Errorf("failed to create aws session: %v", err)
	}

	userAgentHandler := request.NamedHandler{
		Name: "capa/user-agent",
		Fn:   request.MakeAddToUserAgentHandler("eks.control-plane.x-k8s.io", version.Get().String()),
	}

	if params.AWSClients.EC2 == nil {
		ec2Client := ec2.New(session)
		ec2Client.Handlers.Build.PushFrontNamed(userAgentHandler)
		ec2Client.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.EksControlPlane))
		params.AWSClients.EC2 = ec2Client
	}

	if params.AWSClients.ELB == nil {
		elbClient := elb.New(session)
		elbClient.Handlers.Build.PushFrontNamed(userAgentHandler)
		elbClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.EksControlPlane))
		params.AWSClients.ELB = elbClient
	}

	if params.AWSClients.ResourceTagging == nil {
		resourceTagging := resourcegroupstaggingapi.New(session)
		resourceTagging.Handlers.Build.PushFrontNamed(userAgentHandler)
		resourceTagging.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.EksControlPlane))
		params.AWSClients.ResourceTagging = resourceTagging
	}

	if params.AWSClients.SecretsManager == nil {
		sClient := secretsmanager.New(session)
		sClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.EksControlPlane))
		params.AWSClients.SecretsManager = sClient
	}

	if params.AWSClients.EKS == nil {
		eksClient := eks.New(session)
		eksClient.Handlers.Build.PushFrontNamed(userAgentHandler)
		eksClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.EksControlPlane))
		params.AWSClients.EKS = eksClient
	}

	helper, err := patch.NewHelper(params.EksControlPlane, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	networkScope, err := newNetworkScopeFromManagedControlPlane(&params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create network scope from cluster scope")
	}

	return &ManagedControlPlaneScope{
		Logger:          params.Logger,
		client:          params.Client,
		AWSClients:      params.AWSClients,
		Cluster:         params.Cluster,
		EksControlPlane: params.EksControlPlane,
		patchHelper:     helper,
		NetworkScope:    networkScope,
	}, nil
}

func newNetworkScopeFromManagedControlPlane(controlplaneParams *ManagedControlPlaneScopeParams) (*ClusterNetworkScope, error) {
	params := &ClusterNetworkScopeParams{
		Client:         controlplaneParams.Client,
		Logger:         controlplaneParams.Logger,
		Cluster:        controlplaneParams.Cluster,
		AWSClients:     controlplaneParams.AWSClients,
		Target:         interface{}(controlplaneParams.EksControlPlane).(runtime.Object),
		NetworkSpec:    &controlplaneParams.EksControlPlane.Spec.NetworkSpec,
		NetworkStatus:  &controlplaneParams.EksControlPlane.Status.Network,
		Region:         controlplaneParams.EksControlPlane.Spec.Region,
		AdditionalTags: controlplaneParams.EksControlPlane.Labels,
	}

	if controlplaneParams.Cluster.Spec.ClusterNetwork != nil && controlplaneParams.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		params.APIServerPort = controlplaneParams.Cluster.Spec.ClusterNetwork.APIServerPort
	}

	return NewClusterNetworkScope(*params)
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	AWSClients
	Cluster         *clusterv1.Cluster
	EksControlPlane *controlplanev1.EksControlPlane

	NetworkScope *ClusterNetworkScope
}

// Network returns the control plane network object.
func (s *ManagedControlPlaneScope) Network() *infrav1.Network {
	return &s.EksControlPlane.Status.Network
}

// VPC returns the control plane VPC.
func (s *ManagedControlPlaneScope) VPC() *infrav1.VPCSpec {
	return &s.EksControlPlane.Spec.NetworkSpec.VPC
}

// Subnets returns the control plane subnets.
func (s *ManagedControlPlaneScope) Subnets() infrav1.Subnets {
	return s.EksControlPlane.Spec.NetworkSpec.Subnets
}

// SecurityGroups returns the control plane security groups as a map, it creates the map if empty.
func (s *ManagedControlPlaneScope) SecurityGroups() map[infrav1.SecurityGroupRole]infrav1.SecurityGroup {
	return s.EksControlPlane.Status.Network.SecurityGroups
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
	return s.EksControlPlane.Spec.Region
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ManagedControlPlaneScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the control plane configuration and status.
func (s *ManagedControlPlaneScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.EksControlPlane)
}

// Close closes the current scope persisting the control plane configuration and status.
func (s *ManagedControlPlaneScope) Close() error {
	return s.PatchObject()
}

// AdditionalTags returns AdditionalTags from the scope's EksControlPlane. The returned value will never be nil.
func (s *ManagedControlPlaneScope) AdditionalTags() infrav1.Tags {
	if s.EksControlPlane.Spec.AdditionalTags == nil {
		s.EksControlPlane.Spec.AdditionalTags = infrav1.Tags{}
	}

	return s.EksControlPlane.Spec.AdditionalTags.DeepCopy()
}
