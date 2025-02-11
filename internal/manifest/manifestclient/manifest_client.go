package manifestclient

// import (
// 	"sigs.k8s.io/controller-runtime/pkg/client"

// 	"github.com/kyma-project/lifecycle-manager/internal/event"
// )

// const DefaultFieldOwner client.FieldOwner = "declarative.kyma-project.io/applier"

// type ManifestClient struct {
// 	client.Client
// 	event.Event
// }

// func NewManifestClient(event event.Event, kcpClient client.Client) *ManifestClient {
// 	return &ManifestClient{
// 		Event:  event,
// 		Client: kcpClient,
// 	}
// }

// func (m *ManifestClient) UpdateManifest(ctx context.Context, manifest *v1beta2.Manifest) error {
// 	if err := m.Update(ctx, manifest); err != nil {
// 		m.Warning(manifest, "UpdateObject", err)
// 		return fmt.Errorf("failed to update object: %w", err)
// 	}
// 	return nil
// }

// func (m *ManifestClient) PatchStatusIfDiffExist(ctx context.Context, manifest *v1beta2.Manifest,
// 	previousStatus shared.Status,
// ) error {
// 	if hasStatusDiff(manifest.GetStatus(), previousStatus) {
// 		resetNonPatchableField(manifest)
// 		if err := m.Status().Patch(ctx, manifest, client.Apply, client.ForceOwnership, DefaultFieldOwner); err != nil {
// 			m.Warning(manifest, "PatchStatus", err)
// 			return fmt.Errorf("failed to patch status: %w", err)
// 		}
// 	}

// 	return nil
// }

// func hasStatusDiff(first, second shared.Status) bool {
// 	return first.State != second.State || first.LastOperation.Operation != second.LastOperation.Operation
// }

// func resetNonPatchableField(obj client.Object) {
// 	obj.SetUID("")
// 	obj.SetManagedFields(nil)
// 	obj.SetResourceVersion("")
// }
