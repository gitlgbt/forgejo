// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package otel

import (
	"context"
	"fmt"

	"forgejo.org/modules/optional"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Span struct {
	optional.Option[trace.Span]
}

func NewSpan(ctx context.Context, name string) (context.Context, Span) {
	span := Span{optional.None[trace.Span]()}
	var rawSpan trace.Span

	if Setting.Enabled {
		tracer := otel.Tracer("")
		ctx, rawSpan = tracer.Start(ctx, name)
		span = Span{optional.Some(rawSpan)}
	}

	return ctx, span
}

func SpanFromContext(ctx context.Context) Span {
	span := Span{optional.None[trace.Span]()}
	rawSpan := trace.SpanFromContext(ctx)
	if rawSpan.IsRecording() {
		span = Span{optional.Some(rawSpan)}
	}

	return span
}

func (span Span) End() {
	has, rawSpan := span.Get()
	if has {
		rawSpan.End()
	}
}

func (span Span) addAttribute(a attribute.KeyValue) {
	has, rawSpan := span.Get()
	if has {
		rawSpan.SetAttributes(a)
	}
}

func (span Span) AddAttributeBool(k string, v bool) {
	a := attribute.Bool(k, v)
	span.addAttribute(a)
}

func (span Span) AddAttributeString(k string, v string) {
	a := attribute.String(k, v)
	span.addAttribute(a)
}

func (span Span) AddAttributeFloat64(k string, v float64) {
	a := attribute.Float64(k, v)
	span.addAttribute(a)
}

func (span Span) AddAttributeInt(k string, v int) {
	a := attribute.Int(k, v)
	span.addAttribute(a)
}

func (span Span) AddAttributeInt64(k string, v int64) {
	a := attribute.Int64(k, v)
	span.addAttribute(a)
}

func (span Span) AddAttributeAnySliceString(k string, v []any) {
	formatted := []string{}
	for _, v := range v {
		formatted = append(formatted, fmt.Sprintf("%v", v))
	}

	a := attribute.StringSlice(k, formatted)
	span.addAttribute(a)
}
