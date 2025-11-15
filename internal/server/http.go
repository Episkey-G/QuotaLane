package server

import (
	v1 "QuotaLane/api/v1"
	"QuotaLane/internal/conf"
	"QuotaLane/internal/server/middleware"
	"QuotaLane/internal/service"
	pkglog "QuotaLane/pkg/log"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, accountService *service.AccountService, logger log.Logger) *http.Server {
	// 创建增强的日志辅助器
	logHelper := pkglog.NewLogHelper(logger)

	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			middleware.Auth(logHelper),    // 认证中间件：记录 API Key 和 User-Agent
			middleware.Logging(logHelper), // 请求日志中间件：记录请求方法、路径、耗时
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)

	// Register HTTP services
	v1.RegisterAccountServiceHTTPServer(srv, accountService)

	return srv
}
