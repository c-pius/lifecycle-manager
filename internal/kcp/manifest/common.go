package manifest

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const LabelRemovalFinalizer = "label-removal-finalizer"

func setStateToErrorAndReturn(err error, manifest *v1beta2.Manifest) error {
	manifest.SetStatus(manifest.Status.WithState(shared.StateError).WithErr(err))
	return err
}

func patch(ctx context.Context,
	kcp client.Writer,
	manifest *v1beta2.Manifest,
	fieldOwner string,
) error {
	// drop non-patchable fields
	manifest.SetUID("")
	manifest.SetManagedFields(nil)
	manifest.SetResourceVersion("")

	return kcp.Patch(ctx,
		manifest,
		client.Apply,
		client.ForceOwnership,
		client.FieldOwner(fieldOwner))
}
