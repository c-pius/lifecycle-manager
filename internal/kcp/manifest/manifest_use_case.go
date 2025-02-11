package manifest

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type ManifestUseCaseInterface interface {
	CreateModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error
	DeleteModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error)
	RemoveManagedByLabel(ctx context.Context, manifest *v1beta2.Manifest) error
}

type ModuleCRClient interface {
	Create(ctx context.Context, moduleCR unstructured.Unstructured) error
	Delete(ctx context.Context, moduleCR unstructured.Unstructured) (bool, error)
}

type SkrClient interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
}

type ManifestUseCase struct {
	kcp      KcpClient
	skr      SkrClient
	moduleCR ModuleCRClient
}

func NewManifestUseCase(kcp KcpClient, skr SkrClient, moduleCR ModuleCRClient) *ManifestUseCase {
	return &ManifestUseCase{
		kcp:      kcp,
		skr:      skr,
		moduleCR: moduleCR,
	}
}

// CreateModuleCR creates the default Module CR.
// Once created, it adds the CustomResourceManagerFinalizer to the manifest.
func (m *ManifestUseCase) CreateModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error {
	// no default Module CR defined => nothing to do
	if manifest.Spec.Resource == nil {
		return nil
	}

	if err := m.moduleCR.Create(ctx, *manifest.Spec.Resource); err != nil {
		return setStateToErrorAndReturn(fmt.Errorf("failed to create Manifest's Module CR: %w", err), manifest)
	}

	if err := m.addModuleCRFinalizer(ctx, manifest); err != nil {
		return setStateToErrorAndReturn(err, manifest)
	}

	return nil
}

// DeleteModuleCR deletes the default Module CR.
// Once deleted, it removes the CustomResourceManagerFinalizer from the manifest.
// It returns true when the default Module CR is gone and no further reconciliation is needed.
func (m *ManifestUseCase) DeleteModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	// no default Module CR defined => nothing to do
	if manifest.Spec.Resource == nil {
		return true, nil
	}

	deleted, err := m.moduleCR.Delete(ctx, *manifest.Spec.Resource)
	if err != nil {
		return false, setStateToErrorAndReturn(fmt.Errorf("failed to delete Manifest's Module CR: %w", err), manifest)
	}

	if !deleted {
		return false, setStateToErrorAndReturn(errors.New("waiting for Manifest's Module CR do be deleted"), manifest)
	}

	removed, err := m.removeModuleCRFinalizer(ctx, manifest)
	if err != nil {
		return false, setStateToErrorAndReturn(err, manifest)
	}

	// removed the finalizer => we are not done and need another reconciliation
	return !removed, nil
}

func (m *ManifestUseCase) addModuleCRFinalizer(ctx context.Context, manifest *v1beta2.Manifest) error {
	manifestMeta := &v1beta2.Manifest{}
	manifestMeta.SetGroupVersionKind(manifest.GroupVersionKind())
	manifestMeta.SetName(manifest.GetName())
	manifestMeta.SetNamespace(manifest.GetNamespace())
	manifestMeta.SetFinalizers(manifest.GetFinalizers())

	if !controllerutil.AddFinalizer(manifestMeta, CustomResourceManagerFinalizer) {
		return nil
	}

	return patch(ctx, m.kcp, manifestMeta, CustomResourceManagerFinalizer)
}

// removeModuleCRFinalizer removes the CustomResourceManagerFinalizer from the manifest.
// It returns true if the finalizer was actually removed.
func (m *ManifestUseCase) removeModuleCRFinalizer(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	currentManifest := &v1beta2.Manifest{}
	currentManifest.SetGroupVersionKind(manifest.GroupVersionKind())

	if err := m.kcp.Get(ctx, client.ObjectKeyFromObject(manifest), currentManifest); err != nil {
		// Manifest not found => nothing to do
		if util.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get current Manifest CR: %w", err)
	}

	// Finalizer not present => nothing to do
	if !controllerutil.RemoveFinalizer(currentManifest, CustomResourceManagerFinalizer) {
		return false, nil
	}

	if err := m.kcp.Update(ctx, currentManifest, client.FieldOwner(CustomResourceManagerFinalizer)); err != nil {
		return false, fmt.Errorf("failed to remove finalizer from Manifest CR: %w", err)
	}

	return true, nil
}

// RemoveManagedByLabel removes the managed-by label from the synced resources and the default Module CR.
// Once removed, it removes the LabelRemovalFinalizer from the Manifest.
func (m *ManifestUseCase) RemoveManagedByLabel(ctx context.Context, manifest *v1beta2.Manifest) error {
	resourcesError := m.removeManagedByLabelFromSyncedResources(ctx, manifest)
	defaultCRError := m.removeManagedByLabelFromDefaultCR(ctx, manifest)

	if resourcesError != nil || defaultCRError != nil {
		return fmt.Errorf("failed to remove %s label from one or more resources: %w",
			shared.ManagedBy,
			errors.Join(resourcesError, defaultCRError))
	}

	controllerutil.RemoveFinalizer(manifest, LabelRemovalFinalizer)
	if err := m.kcp.Update(ctx, manifest); err != nil {
		return fmt.Errorf("failed to remove %s finalizer from Manifest CR: %w", LabelRemovalFinalizer, err)
	}

	return nil
}

func (m *ManifestUseCase) removeManagedByLabelFromSyncedResources(ctx context.Context, manifestCR *v1beta2.Manifest) error {
	for _, res := range manifestCR.Status.Synced {
		objectKey := client.ObjectKey{
			Name:      res.Name,
			Namespace: res.Namespace,
		}

		obj := constructResource(res)
		if err := m.skr.Get(ctx, objectKey, obj); err != nil {
			return fmt.Errorf("failed to get resource, %w", err)
		}

		if err := m.removeManagedByLabelFromObject(ctx, obj); err != nil {
			return err
		}
	}

	return nil
}

func (m *ManifestUseCase) removeManagedByLabelFromDefaultCR(ctx context.Context, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil {
		return nil
	}

	defaultCR := &unstructured.Unstructured{}
	defaultCR.SetGroupVersionKind(manifest.Spec.Resource.GroupVersionKind())
	err := m.skr.Get(ctx,
		client.ObjectKey{Name: manifest.Spec.Resource.GetName(), Namespace: manifest.Spec.Resource.GetNamespace()},
		defaultCR)

	if err != nil {
		return fmt.Errorf("failed to get default CR, %w", err)
	}

	return m.removeManagedByLabelFromObject(ctx, defaultCR)
}

func (m *ManifestUseCase) removeManagedByLabelFromObject(ctx context.Context, obj *unstructured.Unstructured) error {
	if removeManagedByLabel(obj) {
		if err := m.skr.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to update object: %w", err)
		}
	}

	return nil
}

func constructResource(resource shared.Resource) *unstructured.Unstructured {
	gvk := schema.GroupVersionKind{
		Group:   resource.Group,
		Version: resource.Version,
		Kind:    resource.Kind,
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	return obj
}

func removeManagedByLabel(resource *unstructured.Unstructured) bool {
	labels := resource.GetLabels()
	_, managedByLabelExists := labels[shared.ManagedBy]
	if managedByLabelExists {
		delete(labels, shared.ManagedBy)
	}

	resource.SetLabels(labels)

	return managedByLabelExists
}
