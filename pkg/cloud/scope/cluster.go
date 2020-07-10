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
	"fmt"

	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	Client     client.Client
	Logger     logr.Logger
	Cluster    *clusterv1.Cluster
	AWSCluster *infrav1.AWSCluster
	Session    awsclient.ConfigProvider
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.AWSCluster == nil {
		return nil, errors.New("failed to generate new scope from nil AWSCluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	session, err := sessionForRegion(params.AWSCluster.Spec.Region)
	if err != nil {
		return nil, errors.Errorf("failed to create aws session: %v", err)
	}

	helper, err := patch.NewHelper(params.AWSCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &ClusterScope{
		Logger:      params.Logger,
		client:      params.Client,
		cluster:     params.Cluster,
		awsCluster:  params.AWSCluster,
		patchHelper: helper,
		session:     session,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	cluster    *clusterv1.Cluster
	awsCluster *infrav1.AWSCluster

	session awsclient.ConfigProvider
}

// Network returns the cluster network object.
func (s *ClusterScope) Network() *infrav1.Network {
	return &s.awsCluster.Status.Network
}

// VPC returns the cluster VPC.
func (s *ClusterScope) VPC() *infrav1.VPCSpec {
	return &s.awsCluster.Spec.NetworkSpec.VPC
}

// Subnets returns the cluster subnets.
func (s *ClusterScope) Subnets() infrav1.Subnets {
	return s.awsCluster.Spec.NetworkSpec.Subnets
}

// CNIIngressRules returns the CNI spec ingress rules.
func (s *ClusterScope) CNIIngressRules() infrav1.CNIIngressRules {
	if s.awsCluster.Spec.NetworkSpec.CNI != nil {
		return s.awsCluster.Spec.NetworkSpec.CNI.CNIIngressRules
	}
	return infrav1.CNIIngressRules{}
}

// SecurityGroups returns the cluster security groups as a map, it creates the map if empty.
func (s *ClusterScope) SecurityGroups() map[infrav1.SecurityGroupRole]infrav1.SecurityGroup {
	return s.awsCluster.Status.Network.SecurityGroups
}

// Name returns the cluster name.
func (s *ClusterScope) Name() string {
	return s.cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.cluster.Namespace
}

// Region returns the cluster region.
func (s *ClusterScope) Region() string {
	return s.awsCluster.Spec.Region
}

// ControlPlaneLoadBalancer returns the AWSLoadBalancerSpec
func (s *ClusterScope) ControlPlaneLoadBalancer() *infrav1.AWSLoadBalancerSpec {
	return s.awsCluster.Spec.ControlPlaneLoadBalancer
}

// ControlPlaneLoadBalancerScheme returns the Classic ELB scheme (public or internal facing)
func (s *ClusterScope) ControlPlaneLoadBalancerScheme() infrav1.ClassicELBScheme {
	if s.ControlPlaneLoadBalancer() != nil && s.ControlPlaneLoadBalancer().Scheme != nil {
		return *s.ControlPlaneLoadBalancer().Scheme
	}
	return infrav1.ClassicELBSchemeInternetFacing
}

// ControlPlaneConfigMapName returns the name of the ConfigMap used to
// coordinate the bootstrapping of control plane nodes.
func (s *ClusterScope) ControlPlaneConfigMapName() string {
	return fmt.Sprintf("%s-controlplane", s.cluster.UID)
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.cluster.Name,
	})
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.awsCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.PatchObject()
}

// AdditionalTags returns AdditionalTags from the scope's AWSCluster. The returned value will never be nil.
func (s *ClusterScope) AdditionalTags() infrav1.Tags {
	if s.awsCluster.Spec.AdditionalTags == nil {
		s.awsCluster.Spec.AdditionalTags = infrav1.Tags{}
	}

	return s.awsCluster.Spec.AdditionalTags.DeepCopy()
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ClusterScope) APIServerPort() int32 {
	if s.cluster.Spec.ClusterNetwork != nil && s.cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

// SetFailureDomain sets the infrastructure provider failure domain key to the spec given as input.
func (s *ClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.awsCluster.Status.FailureDomains == nil {
		s.awsCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	s.awsCluster.Status.FailureDomains[id] = spec
}

func (s *ClusterScope) Cluster() *clusterv1.Cluster {
	return s.cluster
}

func (s *ClusterScope) InfraCluster() runtime.Object {
	return s.awsCluster
}

func (s *ClusterScope) Session() awsclient.ConfigProvider {
	return s.session
}

func (s *ClusterScope) Bastion() *infrav1.Bastion {
	return &s.awsCluster.Spec.Bastion
}

func (s *ClusterScope) SetBastionInstance(instance *infrav1.Instance) {
	s.awsCluster.Status.Bastion = instance
}

func (s *ClusterScope) SSHKeyName() *string {
	return s.awsCluster.Spec.SSHKeyName
}

func (s *ClusterScope) SetSubnets(subnets infrav1.Subnets) {
	s.awsCluster.Spec.NetworkSpec.Subnets = subnets
}
