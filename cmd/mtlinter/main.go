// Package main implements the command-line tool for mtlinter.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/mtlinter"
)

func main() {
	singlechecker.Main(mtlinter.Analyzer)
}
