// Package aether provides core application constants and version information
// which are used throughout the application.
package aether

import "github.com/blang/semver"

const (
	// Version is the current version of the application.
	Version = "0.18.0-beta.1"
	// AppName is the name of the application.
	AppName = "aether"
)

// MinVersionCbor is the minimum supported version for CBOR compatibility.
var MinVersionCbor = semver.MustParse("0.12.0")

// MinVersionAgentResponse is the minimum supported version for AgentResponse compatibility.
var MinVersionAgentResponse = semver.MustParse("0.13.0")
