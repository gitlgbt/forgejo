// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package otel

import (
	"context"
	"log"
	"os"

	"forgejo.org/modules/optional"

	"go.opentelemetry.io/contrib/otelconf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// These variables look like settings, and that is because they are!
// Unfortunately, Rob Pike loves his compile times and the Forgejo codebase is
// a bit conveluted because it has grown over a good few years. Due to the
// places in which otel spans would be nice to have in, we arrive in the
// unfortunate position that for example the log module indirectly depends on
// on the otel service. Since log is required by basically everything,
// including the settings, it is not practical to make otel depend on the
// settings (or include logging for that matter) without imort cycles \o/
var Setting = struct {
	Enabled     bool
	ServiceName string
	ConfigFile  optional.Option[string]
}{
	Enabled:     false,
	ServiceName: "Forgejo",
}

func InitOtel(ctx context.Context) (func(ctx context.Context), error) {
	defaultShutdown := func(_ context.Context) {}
	if !Setting.Enabled {
		return defaultShutdown, nil
	}

	sdk, shutdownFunc, err := sdkFromConfig(ctx)
	if err != nil {
		return shutdownFunc, err
	}

	newProvider(sdk)

	return shutdownFunc, err
}

func sdkFromConfig(ctx context.Context) (*otelconf.SDK, func(context.Context), error) {
	defaultShutdown := func(_ context.Context) {}

	conf := &otelconf.OpenTelemetryConfiguration{}
	if Setting.ConfigFile.Has() {
		_, path := Setting.ConfigFile.Get()
		confFile, err := os.ReadFile(path)
		if err != nil {
			return nil, defaultShutdown, err
		}

		conf, err = otelconf.ParseYAML(confFile)
		if err != nil {
			return nil, defaultShutdown, err
		}
	}

	serviceNameAttr := otelconf.AttributeNameValue{
		Name:  "service.name",
		Value: Setting.ServiceName,
	}

	hasName := false
	if conf.Resource != nil {
		for _, a := range conf.Resource.Attributes {
			if a.Name != "service.name" {
				continue
			}

			hasName = true
			break
		}

		if !hasName {
			conf.Resource.Attributes = append(conf.Resource.Attributes, serviceNameAttr)
		}
	} else {
		conf.Resource = &otelconf.Resource{Attributes: []otelconf.AttributeNameValue{serviceNameAttr}}
	}

	sdkconf := otelconf.WithOpenTelemetryConfiguration(*conf)
	sdk, err := otelconf.NewSDK(sdkconf)
	if err != nil {
		return nil, defaultShutdown, err
	}

	shutdownFunc := func(ctx context.Context) {
		err := sdk.Shutdown(ctx)
		if err != nil {
			log.Fatal("Failed to shut down opentelemetry SDK: %s", err.Error())
		}
	}

	return &sdk, shutdownFunc, nil
}

func newProvider(sdk *otelconf.SDK) {
	traceProvider := sdk.TracerProvider()
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}
