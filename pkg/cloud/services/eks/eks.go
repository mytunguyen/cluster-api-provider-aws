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

package eks

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/awserrors"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/record"
)

func (s *Service) ReconcileCluster() error {
	s.scope.V(2).Info("Reconciling EKS cluster")

	cluster, err := s.describeEKSCluster()
	if awserrors.IsNotFound(err) {
		cluster, err = s.createCluster()
	}

	return nil
}

// DeleteCluster deletes an EKS cluster
func (s *Service) DeleteCluster() error {
	cluster, err := s.describeEKSCluster()
	if err != nil {
		if awserrors.IsNotFound(err) {
			s.scope.V(4).Info("eks cluster does not exist")
			return nil
		}
		return errors.Wrap(err, "unable to describe eks cluster")
	}

	err = s.deleteClusterAndWait(cluster)
	if err != nil {
		record.Warnf(s.scope.EKSControlPlane, "FailedDeleteEKSCluster", "Failed to delete EKS cluster %s: %v", cluster.Name, err)
		return errors.Wrap(err, "unable to delete EKS cluster")
	}
	record.Eventf(s.scope.EKSControlPlane, "SuccessfulDeleteEKSCluster", "Deleted EKS Cluster %s", cluster.Name)

	return nil
}

func (s *Service) deleteClusterAndWait(cluster *eks.Cluster) error {
	s.scope.Info("Deleting EKS cluster", "eks-cluster", cluster.Name)

	input := &eks.DeleteClusterInput{
		Name: cluster.Name,
	}
	_, err := s.scope.EKS.DeleteCluster(input)
	if err != nil {
		return errors.Wrapf(err, "failed to request delete of eks cluster %s", cluster.Name)
	}

	waitInput := &eks.DescribeClusterInput{
		Name: cluster.Name,
	}

	err = s.scope.EKS.WaitUntilClusterDeleted(waitInput)
	if err != nil {
		return errors.Wrapf(err, "failed waiting for eks cluster %s to delete", cluster.Name)
	}

	return nil
}

func (s *Service) createCluster() (*eks.Cluster, error) {
	input := &eks.CreateClusterInput{
		Name:               &s.scope.EKSControlPlane.Name,
		ClientRequestToken: aws.String(uuid.New().String()),
		Version:            aws.String(s.scope.EKSControlPlane.Spec.Version),
		Logging:            &eks.Logging{},
		ResourcesVpcConfig: &eks.VpcConfigRequest{},
		RoleArn:            aws.String(s.scope.EKSControlPlane.Spec.RoleArn),
		//Tags: aws.StringMap(),
	}
}

func (s *Service) describeEKSCluster() (*eks.Cluster, error) {
	input := &eks.DescribeClusterInput{
		Name: &s.scope.Cluster.Name,
	}

	out, err := s.scope.EKS.DescribeCluster(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe eks cluster")
	}

	if out.Cluster == nil {
		return nil, awserrors.NewNotFound(errors.New("eks cluster not found"))
	}

	return out.Cluster, nil

}
