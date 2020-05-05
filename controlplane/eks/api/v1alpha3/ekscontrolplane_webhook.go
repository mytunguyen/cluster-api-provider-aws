package v1alpha3

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (in *EksControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(in).Complete()
}

//TODO: add sideeffcts

// +kubebuilder:webhook:verbs=create;update,path=/mutate-controlplane-cluster-x-k8s-io-v1alpha3-ekscontrolplane,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=controlplane.cluster.x-k8s.io,resources=ekscontrolplanes,versions=v1alpha3,name=default.ekscontrolplane.controlplane.cluster.x-k8s.io
// +kubebuilder:webhook:verbs=create;update,path=/validate-controlplane-cluster-x-k8s-io-v1alpha3-ekscontrolplane,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=controlplane.cluster.x-k8s.io,resources=ekscontrolplanes,versions=v1alpha3,name=validation.ekscontrolplane.controlplane.cluster.x-k8s.io

var _ webhook.Defaulter = &EksControlPlane{}
var _ webhook.Validator = &EksControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (in *EksControlPlane) Default() {
}

func (in *EksControlPlane) ValidateCreate() error {
	allErrs := in.validateCommon()

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("EksControlPlane").GroupKind(), in.Name, allErrs)
	}

	return nil
}

func (in *EksControlPlane) ValidateUpdate(old runtime.Object) error {
	//TODO: add checking for paths that can be updated
	return nil
}

func (in *EksControlPlane) ValidateDelete() error {
	return nil
}

func (in *EksControlPlane) validateCommon() (allErrs field.ErrorList) {
	//TODO: add validation
	return allErrs
}
