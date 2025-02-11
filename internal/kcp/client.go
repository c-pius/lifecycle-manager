package kcp

import (
	"github.com/kyma-project/lifecycle-manager/internal/kcp/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KcpClientInterface interface {
	client.Client
	ManifestCR() manifest.ManifestCRInterface
}

type KcpClient struct {
	client.Client
	manifestCR *manifest.ManifestCRClient
}

func NewKcpClient(kcp client.Client, manifestCR *manifest.ManifestCRClient) *KcpClient {
	return &KcpClient{
		Client:     kcp,
		manifestCR: manifestCR,
	}
}

func (k *KcpClient) ManifestCR() manifest.ManifestCRInterface {
	return k.manifestCR
}
