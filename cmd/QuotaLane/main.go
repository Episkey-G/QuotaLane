// Package main is the entry point of QuotaLane service.
// It initializes the Kratos application with gRPC and HTTP servers.
package main

import (
	"flag"
	"os"

	"QuotaLane/internal/conf"
	zapLogger "QuotaLane/pkg/log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs/config.yaml", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()

	// Load configuration using Viper with environment variable and CLI flag support
	bc, err := conf.NewBootstrap(flagconf)
	if err != nil {
		// Use fallback logger before Zap is initialized
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize Zap logger from configuration
	zapLog, err := zapLogger.NewZapLogger(bc.Log)
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	defer zapLog.Sync()

	// Create Kratos adapter for Zap logger
	logger := zapLogger.NewKratosAdapter(zapLog)

	// Add context fields to logger
	logger = log.With(logger,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	// Log startup configuration
	log.NewHelper(logger).Infow(
		"msg", "QuotaLane service starting",
		"log.level", bc.Log.Level,
		"log.format", bc.Log.Format,
		"log.env", bc.Log.Env,
		"log.output_file", bc.Log.OutputFile,
	)

	app, cleanup, err := wireApp(bc.Server, bc.Data, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
