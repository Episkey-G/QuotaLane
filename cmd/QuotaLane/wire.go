//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"fmt"

	"QuotaLane/internal/biz"
	"QuotaLane/internal/conf"
	"QuotaLane/internal/data"
	"QuotaLane/internal/server"
	"QuotaLane/internal/service"
	"QuotaLane/pkg/crypto"
	"QuotaLane/pkg/oauth"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// AppComponents holds the application and its dependencies.
type AppComponents struct {
	App       *kratos.App
	AccountUC *biz.AccountUsecase
}

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *conf.Auth, log.Logger) (*AppComponents, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		oauth.ProviderSet,
		newCryptoService,
		newApp,
		wire.Struct(new(AppComponents), "*"),
	))
}

// newCryptoService creates AES crypto service from config.
func newCryptoService(auth *conf.Auth) (*crypto.AESCrypto, error) {
	if auth == nil || auth.Encryption == nil {
		return nil, fmt.Errorf("encryption configuration is required but not found in auth config")
	}
	if len(auth.Encryption.Key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes, got %d bytes", len(auth.Encryption.Key))
	}
	return crypto.NewAESCrypto([]byte(auth.Encryption.Key))
}
