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
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/version"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedClusterScopeParams defines the input parameters used to create a new Scope.
type ManagedClusterScopeParams struct {
	AWSClients
	Client            client.Client
	Logger            logr.Logger
	Cluster           *clusterv1.Cluster
	AWSManagedCluster *infrav1.AWSManagedCluster
}

// NewManagedClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedClusterScope(params ManagedClusterScopeParams) (*ManagedClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.AWSManagedCluster == nil {
		return nil, errors.New("failed to generate new scope from nil AWSManagedCluster")
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
		Fn:   request.MakeAddToUserAgentHandler("aws.cluster.x-k8s.io", version.Get().String()),
	}

	if params.AWSClients.EC2 == nil {
		ec2Client := ec2.New(session)
		ec2Client.Handlers.Build.PushFrontNamed(userAgentHandler)
		ec2Client.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.AWSManagedCluster))
		params.AWSClients.EC2 = ec2Client
	}

	if params.AWSClients.ELB == nil {
		elbClient := elb.New(session)
		elbClient.Handlers.Build.PushFrontNamed(userAgentHandler)
		elbClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.AWSManagedCluster))
		params.AWSClients.ELB = elbClient
	}

	if params.AWSClients.ResourceTagging == nil {
		resourceTagging := resourcegroupstaggingapi.New(session)
		resourceTagging.Handlers.Build.PushFrontNamed(userAgentHandler)
		resourceTagging.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.AWSManagedCluster))
		params.AWSClients.ResourceTagging = resourceTagging
	}

	if params.AWSClients.SecretsManager == nil {
		sClient := secretsmanager.New(session)
		sClient.Handlers.Complete.PushBack(recordAWSPermissionsIssue(params.AWSManagedCluster))
		params.AWSClients.SecretsManager = sClient
	}

	helper, err := patch.NewHelper(params.AWSManagedCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	networkScope, err := newNetworkScopeFromManagedCluster(&params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create network scope from managed cluster scope")
	}

	return &ManagedClusterScope{
		Logger:            params.Logger,
		client:            params.Client,
		AWSClients:        params.AWSClients,
		Cluster:           params.Cluster,
		AWSManagedCluster: params.AWSManagedCluster,
		patchHelper:       helper,
		NetworkScope:      networkScope,
	}, nil
}

func newNetworkScopeFromManagedCluster(clusterParams *ManagedClusterScopeParams) (*ClusterNetworkScope, error) {
	params := &ClusterNetworkScopeParams{
		Client:         clusterParams.Client,
		Logger:         clusterParams.Logger,
		Cluster:        clusterParams.Cluster,
		Target:         interface{}(clusterParams.AWSManagedCluster).(runtime.Object),
		NetworkSpec:    &clusterParams.AWSManagedCluster.Spec.NetworkSpec,
		NetworkStatus:  &clusterParams.AWSManagedCluster.Status.Network,
		Region:         clusterParams.AWSManagedCluster.Spec.Region,
		AWSClients:     clusterParams.AWSClients,
		AdditionalTags: clusterParams.AWSManagedCluster.Labels,
	}

	if clusterParams.Cluster.Spec.ClusterNetwork != nil && clusterParams.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		params.APIServerPort = clusterParams.Cluster.Spec.ClusterNetwork.APIServerPort
	}

	return NewClusterNetworkScope(*params)
}

// ManagedClusterScope defines the basic context for an actuator to operate upon.
type ManagedClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	AWSClients
	Cluster           *clusterv1.Cluster
	AWSManagedCluster *infrav1.AWSManagedCluster

	NetworkScope *ClusterNetworkScope
}

// Network returns the control plane network object.
func (s *ManagedClusterScope) Network() *infrav1.Network {
	return &s.AWSManagedCluster.Status.Network
}

// VPC returns the control plane VPC.
func (s *ManagedClusterScope) VPC() *infrav1.VPCSpec {
	return &s.AWSManagedCluster.Spec.NetworkSpec.VPC
}

// Subnets returns the control plane subnets.
func (s *ManagedClusterScope) Subnets() infrav1.Subnets {
	return s.AWSManagedCluster.Spec.NetworkSpec.Subnets
}

// SecurityGroups returns the control plane security groups as a map, it creates the map if empty.
func (s *ManagedClusterScope) SecurityGroups() map[infrav1.SecurityGroupRole]infrav1.SecurityGroup {
	return s.AWSManagedCluster.Status.Network.SecurityGroups
}

// Name returns the cluster name.
func (s *ManagedClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ManagedClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// Region returns the cluster region.
func (s *ManagedClusterScope) Region() string {
	return s.AWSManagedCluster.Spec.Region
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ManagedClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the control plane configuration and status.
func (s *ManagedClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.AWSManagedCluster)
}

// Close closes the current scope persisting the control plane configuration and status.
func (s *ManagedClusterScope) Close() error {
	return s.PatchObject()
}

// AdditionalTags returns AdditionalTags from the scope's EksControlPlane. The returned value will never be nil.
func (s *ManagedClusterScope) AdditionalTags() infrav1.Tags {
	if s.AWSManagedCluster.Spec.AdditionalTags == nil {
		s.AWSManagedCluster.Spec.AdditionalTags = infrav1.Tags{}
	}

	return s.AWSManagedCluster.Spec.AdditionalTags.DeepCopy()
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ManagedClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

// SetFailureDomain sets the infrastructure provider failure domain key to the spec given as input.
func (s *ManagedClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.AWSManagedCluster.Status.FailureDomains == nil {
		s.AWSManagedCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	s.AWSManagedCluster.Status.FailureDomains[id] = spec
}
