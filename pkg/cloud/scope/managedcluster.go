/*
Copyright 2018 The Kubernetes Authors.

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

	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedClusterScopeParams defines the input parameters used to create a new Scope.
type ManagedClusterScopeParams struct {
	Client            client.Client
	Logger            logr.Logger
	Cluster           *clusterv1.Cluster
	AWSManagedCluster *infrav1exp.AWSManagedCluster
	Controlplane      *infrav1exp.AWSManagedControlPlane
	Session           awsclient.ConfigProvider
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
	if params.Controlplane == nil {
		return nil, errors.New("failed to generate new scope from nil ControlPlane")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	session, err := sessionForRegion(params.AWSManagedCluster.Spec.Region)
	if err != nil {
		return nil, errors.Errorf("failed to create aws session: %v", err)
	}

	helper, err := patch.NewHelper(params.AWSManagedCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &ManagedClusterScope{
		Logger:            params.Logger,
		client:            params.Client,
		cluster:           params.Cluster,
		awsManagedCluster: params.AWSManagedCluster,
		controlplane:      params.Controlplane,
		patchHelper:       helper,
		session:           session,
	}, nil
}

// ManagedClusterScope defines the basic context for an actuator to operate upon.
type ManagedClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	cluster           *clusterv1.Cluster
	awsManagedCluster *infrav1exp.AWSManagedCluster
	controlplane      *infrav1exp.AWSManagedControlPlane

	session awsclient.ConfigProvider
}

// Network returns the cluster network object.
func (s *ManagedClusterScope) Network() *infrav1.Network {
	return &s.awsManagedCluster.Status.Network
}

// VPC returns the cluster VPC.
func (s *ManagedClusterScope) VPC() *infrav1.VPCSpec {
	return &s.awsManagedCluster.Spec.NetworkSpec.VPC
}

// Subnets returns the cluster subnets.
func (s *ManagedClusterScope) Subnets() infrav1.Subnets {
	return s.awsManagedCluster.Spec.NetworkSpec.Subnets
}

// CNIIngressRules returns the CNI spec ingress rules.
func (s *ManagedClusterScope) CNIIngressRules() infrav1.CNIIngressRules {
	if s.awsManagedCluster.Spec.NetworkSpec.CNI != nil {
		return s.awsManagedCluster.Spec.NetworkSpec.CNI.CNIIngressRules
	}
	return infrav1.CNIIngressRules{}
}

// SecurityGroups returns the cluster security groups as a map, it creates the map if empty.
func (s *ManagedClusterScope) SecurityGroups() map[infrav1.SecurityGroupRole]infrav1.SecurityGroup {
	return s.awsManagedCluster.Status.Network.SecurityGroups
}

// Name returns the cluster name.
func (s *ManagedClusterScope) Name() string {
	return s.cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ManagedClusterScope) Namespace() string {
	return s.cluster.Namespace
}

// Region returns the cluster region.
func (s *ManagedClusterScope) Region() string {
	return s.awsManagedCluster.Spec.Region
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ManagedClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.cluster.Name,
	})
}

// PatchObject persists the cluster configuration and status.
func (s *ManagedClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.awsManagedCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ManagedClusterScope) Close() error {
	return s.PatchObject()
}

// AdditionalTags returns AdditionalTags from the scope's AWSCluster. The returned value will never be nil.
func (s *ManagedClusterScope) AdditionalTags() infrav1.Tags {
	if s.awsManagedCluster.Spec.AdditionalTags == nil {
		s.awsManagedCluster.Spec.AdditionalTags = infrav1.Tags{}
	}

	return s.awsManagedCluster.Spec.AdditionalTags.DeepCopy()
}

// SetFailureDomain sets the infrastructure provider failure domain key to the spec given as input.
func (s *ManagedClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.awsManagedCluster.Status.FailureDomains == nil {
		s.awsManagedCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	s.awsManagedCluster.Status.FailureDomains[id] = spec
}

func (s *ManagedClusterScope) APIServerPort() int32 {
	return 443
}

func (s *ManagedClusterScope) Cluster() *clusterv1.Cluster {
	return s.cluster
}

func (s *ManagedClusterScope) InfraCluster() runtime.Object {
	return s.awsManagedCluster
}

func (s *ManagedClusterScope) Session() awsclient.ConfigProvider {
	return s.session
}

func (s *ManagedClusterScope) Bastion() *infrav1.Bastion {
	return &s.awsManagedCluster.Spec.Bastion
}

func (s *ManagedClusterScope) SetBastionInstance(instance *infrav1.Instance) {
	s.awsManagedCluster.Status.Bastion = instance
}

func (s *ManagedClusterScope) SSHKeyName() *string {
	return s.awsManagedCluster.Spec.SSHKeyName
}

func (s *ManagedClusterScope) SetSubnets(subnets infrav1.Subnets) {
	s.awsManagedCluster.Spec.NetworkSpec.Subnets = subnets
}

func (s *ManagedClusterScope) ControlPlane() *infrav1exp.AWSManagedControlPlane {
	return s.controlplane
}
