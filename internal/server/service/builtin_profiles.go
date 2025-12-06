// Package service provides business logic services
package service

import (
	"embed"
)

//go:embed builtin/*.json
var builtinProfiles embed.FS

// GetBuiltinProfiles returns the embedded builtin profiles filesystem
func GetBuiltinProfiles() embed.FS {
	return builtinProfiles
}
