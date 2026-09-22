// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log/slog"
	"os"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	flagd "github.com/open-feature/go-sdk-contrib/providers/flagd/pkg"
	"github.com/open-feature/go-sdk/openfeature"
)

// crossSellClient reads crossSellRecommendation. The generated `flags` package is produced by
// the OpenFeature CLI from the flag manifest and must not be hand-edited, so this flag - which
// belongs to this fork, not upstream - is read through a client of its own.
var crossSellClient = openfeature.NewClient("product-catalog-cross-sell")

// newFlagProvider picks the OpenFeature provider from the environment.
//
// DevCycle when DEVCYCLE_SERVER_SDK_KEY is set, flagd otherwise. Nothing that reads a flag
// knows which one answered - that is the whole point of OpenFeature, and it is what lets the
// same image run in a cluster with DevCycle and on a laptop with flagd and no SDK key.
//
// The returned name is for the log line only. A demo that silently falls back to flagd because
// a secret failed to mount would show the flag "not enabled" and look like a product defect,
// so which provider answered has to be visible at startup.
func newFlagProvider(logger *slog.Logger) (openfeature.FeatureProvider, string, error) {
	key := os.Getenv("DEVCYCLE_SERVER_SDK_KEY")
	if key == "" {
		provider, err := flagd.NewProvider()
		return provider, "flagd", err
	}

	// Local bucketing: the SDK pulls the config and evaluates in-process, so a flag read costs
	// no network call on the request path. GetProduct evaluates per request under load-generator
	// traffic, and cloud bucketing would put a hop to DevCycle inside every product lookup.
	client, err := devcycle.NewClient(key, &devcycle.Options{
		EnableCloudBucketing: false,
	})
	if err != nil {
		// Fall back rather than fail: the catalog serving products matters more than the flag
		// source, and the log line above says which one is live.
		logger.Error("DevCycle client failed; falling back to flagd", slog.Any("error", err))
		provider, ferr := flagd.NewProvider()
		return provider, "flagd (DevCycle failed)", ferr
	}

	return client.OpenFeatureProvider(), "devcycle", nil
}
