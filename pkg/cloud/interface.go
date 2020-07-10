package cloud

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	awsclient "github.com/aws/aws-sdk-go/aws/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

type Session interface {
	Session() awsclient.ConfigProvider
}

type ClusterScoper interface {
	logr.Logger
	Session

	Name() string
	Namespace() string
	Region() string

	Cluster() *clusterv1.Cluster
	InfraCluster() runtime.Object

	Network() *infrav1.Network
	VPC() *infrav1.VPCSpec
	Subnets() infrav1.Subnets
	SetSubnets(subnets infrav1.Subnets)
	CNIIngressRules() infrav1.CNIIngressRules
	SecurityGroups() map[infrav1.SecurityGroupRole]infrav1.SecurityGroup
	ListOptionsLabelSelector() client.ListOption
	PatchObject() error
	Close() error
	APIServerPort() int32
	AdditionalTags() infrav1.Tags
	SetFailureDomain(id string, spec clusterv1.FailureDomainSpec)
	Bastion() *infrav1.Bastion
	SetBastionInstance(instance *infrav1.Instance)
	SSHKeyName() *string
}
