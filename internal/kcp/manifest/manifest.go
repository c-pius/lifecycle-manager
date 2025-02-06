package manifest

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/mandelsoft/goutils/errors"
)

const CustomResourceManagerFinalizer = "resource.kyma-project.io/finalizer"

type ManifestCRInterface interface {
	DeleteModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error)
}

type KcpClient interface {
	client.Client
}

type ModuleCRClient interface {
	Delete(ctx context.Context, moduleCR unstructured.Unstructured) (bool, error)
}

type ManifestCRClient struct {
	kcp      KcpClient
	moduleCR ModuleCRClient
}

func NewManifestClient(kcp client.Client, moduleCR ModuleCRClient) *ManifestCRClient {
	return &ManifestCRClient{
		kcp:      kcp,
		moduleCR: moduleCR,
	}
}

// DeleteModuleCR deletes the default Module CR.
// Once deleted, it removes the CustomResourceManagerFinalizer from the manifest.
// It returns true when no further reconciliation is needed.
func (m *ManifestCRClient) DeleteModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	// no default Module CR defined => nothing to do
	if manifest.Spec.Resource == nil {
		return true, nil
	}

	deleted, err := m.moduleCR.Delete(ctx, *manifest.Spec.Resource)
	if err != nil {
		return false, fmt.Errorf("failed to delete manifest's Module CR: %w", err)
	}

	if !deleted {
		return false, errors.New("waiting for Manifest's Module CR do be deleted")
	}

	removed, err := m.removeModuleCRFinalizer(ctx, manifest)
	if err != nil {
		return false, err
	}

	// removed the finalizer => we are not done and need another reconciliation
	return !removed, nil
}

// removeModuleCRFinalizer removes the CustomResourceManagerFinalizer from the manifest.
// It returns true if the finalizer was actually removed.
func (m *ManifestCRClient) removeModuleCRFinalizer(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	currentManifest := &v1beta2.Manifest{}
	currentManifest.SetGroupVersionKind(manifest.GroupVersionKind())

	if err := m.kcp.Get(ctx, client.ObjectKeyFromObject(manifest), currentManifest); err != nil {
		// Manifest not found => nothing to do
		if util.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get current manifest: %w", err)
	}

	// Finalizer not present => nothing to do
	if !controllerutil.RemoveFinalizer(currentManifest, CustomResourceManagerFinalizer) {
		return false, nil
	}

	if err := m.kcp.Update(ctx, currentManifest, client.FieldOwner(CustomResourceManagerFinalizer)); err != nil {
		return false, fmt.Errorf("failed to remove finalizer from manifest: %w", err)
	}

	return true, nil
}
