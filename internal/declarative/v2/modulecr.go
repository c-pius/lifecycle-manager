package v2

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type ModuleCR struct {
	skr client.Client
	kcp client.Client
}

func NewModuleCR(skr client.Client, kcp client.Client) *ModuleCR {
	return &ModuleCR{
		skr: skr,
		kcp: kcp,
	}
}

func (m *ModuleCR) GetModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (*unstructured.Unstructured, error) {
	resourceCR := &unstructured.Unstructured{}
	name := manifest.Spec.Resource.GetName()
	namespace := manifest.Spec.Resource.GetNamespace()
	gvk := manifest.Spec.Resource.GroupVersionKind()

	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	if err := m.skr.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		return nil, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR, nil
}

// DeleteModuleCR deletes the module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
func (m *ModuleCR) DeleteModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error {
	crDeleted, err := m.deleteCR(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithErr(err))
		return err
	}
	if crDeleted {
		if err := finalizer.RemoveCRFinalizer(ctx, m.kcp, manifest); err != nil {
			manifest.SetStatus(manifest.GetStatus().WithErr(err))
			return err
		}
	}
	return nil
}

func (m *ModuleCR) deleteCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	if manifest.Spec.Resource == nil {
		return false, nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	propagation := apimetav1.DeletePropagationBackground
	err := m.skr.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})
	if util.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to fetch resource: %w", err)
	}
	return false, nil
}

// SyncModuleCR sync the manifest default custom resource status in the cluster, if not available it created the resource.
// It is used to provide the controller with default data in the Runtime.
func (m *ModuleCR) SyncModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	resource.SetLabels(collections.MergeMaps(resource.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	if err := m.skr.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil && util.IsNotFound(err) {
		if !manifest.GetDeletionTimestamp().IsZero() {
			return nil
		}
		if err := m.skr.Create(ctx, resource,
			client.FieldOwner(finalizer.CustomResourceManagerFinalizer)); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create resource: %w", err)
		}
	}
	return nil
}

func (m *ModuleCR) CheckModuleCRDeletion(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	if manifest.Spec.Resource == nil {
		return true, nil
	}

	resourceCR, err := m.GetModuleCR(ctx, manifest)
	if err != nil {
		if util.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR == nil, nil
}
