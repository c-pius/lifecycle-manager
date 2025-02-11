package skr

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/kcp/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type ModuleCRInterface interface {
	Create(ctx context.Context, moduleCR unstructured.Unstructured) error
	Delete(ctx context.Context, moduleCR unstructured.Unstructured) (bool, error)
	Exists(ctx context.Context, moduleCR unstructured.Unstructured) (bool, error)
}

type SkrClient interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
}

var ErrNoResourceDefined = errors.New("no resource defined in the manifest")

var deletePropagationBackground = apimetav1.DeletePropagationBackground

type ModuleCRClient struct {
	skr SkrClient
}

func NewModuleCRClient(skr client.Client) *ModuleCRClient {
	return &ModuleCRClient{
		skr: skr,
	}
}

// Delete deletes the default Module CR.
// It uses DeletePropagationBackground.
// It returns true only if the resource was not found.
func (m *ModuleCRClient) Delete(ctx context.Context, moduleCR unstructured.Unstructured) (bool, error) {
	err := m.skr.Delete(ctx, &moduleCR, &client.DeleteOptions{PropagationPolicy: &deletePropagationBackground})

	if util.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to delete default Module CR: %w", err)
	}

	return false, nil
}

// Create creates the default Module CR.
func (m *ModuleCRClient) Create(ctx context.Context, moduleCR unstructured.Unstructured) error {
	moduleCR.SetLabels(collections.MergeMaps(
		moduleCR.GetLabels(),
		map[string]string{
			shared.ManagedBy: shared.ManagedByLabelValue,
		}))

	// this is likely the reason for https://github.com/kyma-project/lifecycle-manager/issues/2234
	// if it exists, we should consider to use an upsert instead
	err := m.skr.Create(ctx, &moduleCR, client.FieldOwner(manifest.CustomResourceManagerFinalizer))

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create default Module CR: %w", err)
	}

	return nil
}

// Exists checks if the default Module CR exists.
func (m *ModuleCRClient) Exists(ctx context.Context, moduleCR unstructured.Unstructured) (bool, error) {
	err := m.skr.Get(ctx,
		client.ObjectKeyFromObject(&moduleCR),
		&moduleCR)

	if util.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("failed to fetch default Module CR: %w", err)
	}

	return true, nil
}
