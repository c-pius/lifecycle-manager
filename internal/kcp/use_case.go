package kcp

import (
	"github.com/kyma-project/lifecycle-manager/internal/kcp/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KcpUseCaseInterface interface {
	ManifestUseCase() manifest.ManifestUseCaseInterface
}

type KcpUseCase struct {
	manifestUseCase *manifest.ManifestUseCase
}

func NewKcpUseCase(kcp client.Client,
	manifestUseCase *manifest.ManifestUseCase) *KcpUseCase {
	return &KcpUseCase{
		manifestUseCase: manifestUseCase,
	}
}

func (k *KcpUseCase) ManifestUseCase() manifest.ManifestUseCaseInterface {
	return k.manifestUseCase
}
