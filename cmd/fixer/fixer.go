// Package main provides the entrypoint for the fixer
package main

import (
	"context"
	"flag"
	"os"

	"github.com/artificialinc/cm-429-fixer/pkg/cm"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	logLevel = os.Getenv("LOG_LEVEL")
)

func init() {
	if logLevel == "" {
		logLevel = "info"
	}
}

func main() {
	k8sContext := flag.String("k8s-context", "", "Kubernetes context to use")
	flag.Parse()

	zapCfg := zap.NewProductionConfig()
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		panic(err)
	}
	zapCfg.Level.SetLevel(level)
	logger, err := zapCfg.Build()
	if err != nil {
		zap.S().Fatalf("Failed to get logger: %v", err)
	}

	watcher := cm.NewWatcher(
		cm.WithLogger(zapr.NewLogger(logger).WithName("watcher")),
		cm.WithClient(cm.GetLocalClient(&cm.ClientOpts{Context: *k8sContext})),
	)

	ctx := context.Background()

	watcher.Run(ctx, make(chan bool))
}
