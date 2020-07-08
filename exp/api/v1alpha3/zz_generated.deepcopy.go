// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha3

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	apiv1alpha3 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	cluster_apiapiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedCluster) DeepCopyInto(out *AWSManagedCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedCluster.
func (in *AWSManagedCluster) DeepCopy() *AWSManagedCluster {
	if in == nil {
		return nil
	}
	out := new(AWSManagedCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AWSManagedCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedClusterList) DeepCopyInto(out *AWSManagedClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AWSManagedCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedClusterList.
func (in *AWSManagedClusterList) DeepCopy() *AWSManagedClusterList {
	if in == nil {
		return nil
	}
	out := new(AWSManagedClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AWSManagedClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedClusterSpec) DeepCopyInto(out *AWSManagedClusterSpec) {
	*out = *in
	in.NetworkSpec.DeepCopyInto(&out.NetworkSpec)
	if in.SSHKeyName != nil {
		in, out := &in.SSHKeyName, &out.SSHKeyName
		*out = new(string)
		**out = **in
	}
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	if in.AdditionalTags != nil {
		in, out := &in.AdditionalTags, &out.AdditionalTags
		*out = make(apiv1alpha3.Tags, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Bastion = in.Bastion
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedClusterSpec.
func (in *AWSManagedClusterSpec) DeepCopy() *AWSManagedClusterSpec {
	if in == nil {
		return nil
	}
	out := new(AWSManagedClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedClusterStatus) DeepCopyInto(out *AWSManagedClusterStatus) {
	*out = *in
	in.Network.DeepCopyInto(&out.Network)
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make(cluster_apiapiv1alpha3.FailureDomains, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Bastion != nil {
		in, out := &in.Bastion, &out.Bastion
		*out = new(apiv1alpha3.Instance)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedClusterStatus.
func (in *AWSManagedClusterStatus) DeepCopy() *AWSManagedClusterStatus {
	if in == nil {
		return nil
	}
	out := new(AWSManagedClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedControlPlane) DeepCopyInto(out *AWSManagedControlPlane) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedControlPlane.
func (in *AWSManagedControlPlane) DeepCopy() *AWSManagedControlPlane {
	if in == nil {
		return nil
	}
	out := new(AWSManagedControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AWSManagedControlPlane) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedControlPlaneList) DeepCopyInto(out *AWSManagedControlPlaneList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AWSManagedControlPlane, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedControlPlaneList.
func (in *AWSManagedControlPlaneList) DeepCopy() *AWSManagedControlPlaneList {
	if in == nil {
		return nil
	}
	out := new(AWSManagedControlPlaneList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AWSManagedControlPlaneList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedControlPlaneSpec) DeepCopyInto(out *AWSManagedControlPlaneSpec) {
	*out = *in
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
	if in.RoleName != nil {
		in, out := &in.RoleName, &out.RoleName
		*out = new(string)
		**out = **in
	}
	if in.RoleAdditionalPolicies != nil {
		in, out := &in.RoleAdditionalPolicies, &out.RoleAdditionalPolicies
		*out = new([]string)
		if **in != nil {
			in, out := *in, *out
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
	}
	if in.EncryptionConfig != nil {
		in, out := &in.EncryptionConfig, &out.EncryptionConfig
		*out = new([]EncryptionConfig)
		if **in != nil {
			in, out := *in, *out
			*out = make([]EncryptionConfig, len(*in))
			for i := range *in {
				(*in)[i].DeepCopyInto(&(*out)[i])
			}
		}
	}
	if in.AdditionalTags != nil {
		in, out := &in.AdditionalTags, &out.AdditionalTags
		*out = make(apiv1alpha3.Tags, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Private != nil {
		in, out := &in.Private, &out.Private
		*out = new(bool)
		**out = **in
	}
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedControlPlaneSpec.
func (in *AWSManagedControlPlaneSpec) DeepCopy() *AWSManagedControlPlaneSpec {
	if in == nil {
		return nil
	}
	out := new(AWSManagedControlPlaneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSManagedControlPlaneStatus) DeepCopyInto(out *AWSManagedControlPlaneStatus) {
	*out = *in
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSManagedControlPlaneStatus.
func (in *AWSManagedControlPlaneStatus) DeepCopy() *AWSManagedControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(AWSManagedControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EksCluster) DeepCopyInto(out *EksCluster) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EksCluster.
func (in *EksCluster) DeepCopy() *EksCluster {
	if in == nil {
		return nil
	}
	out := new(EksCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EncryptionConfig) DeepCopyInto(out *EncryptionConfig) {
	*out = *in
	if in.Provider != nil {
		in, out := &in.Provider, &out.Provider
		*out = new(apiv1alpha3.AWSResourceReference)
		(*in).DeepCopyInto(*out)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]*string, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(string)
				**out = **in
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EncryptionConfig.
func (in *EncryptionConfig) DeepCopy() *EncryptionConfig {
	if in == nil {
		return nil
	}
	out := new(EncryptionConfig)
	in.DeepCopyInto(out)
	return out
}
