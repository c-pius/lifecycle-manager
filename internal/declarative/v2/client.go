package v2

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
)

type Client interface {
	resource.RESTClientGetter
	skrresources.ResourceInfoConverter

	client.Client
	ModuleCRClient
}

type ModuleCRClient interface {
	GetModuleCR(ctx context.Context, manifest *v1beta2.Manifest) (*unstructured.Unstructured, error)
	DeleteModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error
	SyncModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error
	CheckModuleCRDeletion(ctx context.Context, manifest *v1beta2.Manifest) (bool, error)
}
