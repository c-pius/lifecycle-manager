package manifest

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	CustomResourceManagerFinalizer = "resource.kyma-project.io/finalizer"
	DefaultFinalizer               = "declarative.kyma-project.io/finalizer"

	DefaultFieldOwner client.FieldOwner = "declarative.kyma-project.io/applier"
)

var mandatoryFinalizers = []string{DefaultFinalizer, LabelRemovalFinalizer}

type ManifestCRInterface interface {
	Update(ctx context.Context, manifest *v1beta2.Manifest) error
	UpdateStatus(ctx context.Context, manifest *v1beta2.Manifest, oldStatus shared.Status) error
	AddMandatoryFinalizers(ctx context.Context, manifest *v1beta2.Manifest) (bool, error)
	RemoveMandatoryFinalizers(ctx context.Context, manifest *v1beta2.Manifest) (bool, error)
	RemoveAllFinalizers(ctx context.Context, manifest *v1beta2.Manifest) (bool, error)
}

type KcpClient interface {
	client.Client
}

type ManifestCRClient struct {
	kcp KcpClient
}

func NewManifestCRClient(kcp client.Client) *ManifestCRClient {
	return &ManifestCRClient{
		kcp: kcp,
	}
}

// Update updates the manifest.
func (m *ManifestCRClient) Update(ctx context.Context, manifest *v1beta2.Manifest) error {
	if err := m.kcp.Update(ctx, manifest); err != nil {
		return fmt.Errorf("failed to update Manifest: %w", err)
	}
	return nil
}

// AddMandatoryFinalizers adds the mandatory finalizers to the manifest.
// It returns true if the finalizers were actually added.
func (m *ManifestCRClient) AddMandatoryFinalizers(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	numFinalizers := len(manifest.GetFinalizers())

	for _, finalizer := range mandatoryFinalizers {
		controllerutil.AddFinalizer(manifest, finalizer)
	}

	if numFinalizers == len(manifest.GetFinalizers()) {
		return false, nil
	}

	if err := patch(ctx,
		m.kcp,
		manifest,
		DefaultFinalizer); err != nil {
		return false, setStateToErrorAndReturn(fmt.Errorf("failed to add mandatory finalizers to Manifest CR: %w", err), manifest)
	}

	return true, nil
}

// UpdateStatus updates the status of the Manifest if it differs from the old status.
func (m *ManifestCRClient) UpdateStatus(ctx context.Context, manifest *v1beta2.Manifest, oldStatus shared.Status) error {
	newStatus := manifest.GetStatus()

	if !hasStatusDiff(newStatus, oldStatus) {
		return nil
	}

	// drop non-patchable fields
	manifest.SetUID("")
	manifest.SetManagedFields(nil)
	manifest.SetResourceVersion("")

	if err := m.kcp.Status().Patch(ctx,
		manifest,
		client.Apply,
		client.ForceOwnership,
		DefaultFieldOwner); err != nil {
		return fmt.Errorf("failed to update Manifest CR status: %w", err)
	}

	return nil
}

// RemoveRequiredFinalizers removes the mandatory finalizers from the Manifest CR.
// It returns true if the finalizers were actually removed.
func (m *ManifestCRClient) RemoveMandatoryFinalizers(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	numFinalizers := len(manifest.GetFinalizers())

	for _, finalizer := range mandatoryFinalizers {
		controllerutil.RemoveFinalizer(manifest, finalizer)
	}

	if numFinalizers == len(manifest.GetFinalizers()) {
		return false, nil
	}

	if err := m.Update(ctx, manifest); err != nil {
		return false, setStateToErrorAndReturn(fmt.Errorf("failed to remove mandatory finalizers from Manifest CR: %w", err), manifest)
	}

	return true, nil
}

// RemoveAllFinalizers removes all finalizers from the Manifest CR.
// It returns true if the finalizers were actually removed.
func (m *ManifestCRClient) RemoveAllFinalizers(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	if len(manifest.GetFinalizers()) == 0 {
		return false, nil
	}

	manifest.SetFinalizers([]string{})

	if err := m.Update(ctx, manifest); err != nil {
		return false, setStateToErrorAndReturn(fmt.Errorf("failed to remove all finalizers from Manifest CR: %w", err), manifest)
	}

	return true, nil
}

func hasStatusDiff(first, second shared.Status) bool {
	return first.State != second.State || first.LastOperation.Operation != second.LastOperation.Operation
}
