//+build tools

package influxdb

import (
	_ "github.com/benbjohnson/tmpl"
	_ "github.com/editorconfig-checker/editorconfig-checker/cmd/editorconfig-checker"
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "github.com/gogo/protobuf/protoc-gen-gogofaster"
	_ "github.com/influxdata/pkg-config"
	_ "github.com/kevinburke/go-bindata/go-bindata"
	_ "github.com/mna/pigeon"
	_ "golang.org/x/tools/cmd/goimports"
	_ "golang.org/x/tools/cmd/stringer"
	_ "honnef.co/go/tools/cmd/staticcheck"
)

// This package is a workaround for adding additional paths to the go.mod file
// and ensuring they stay there. The build tag ensures this file never gets
// compiled, but the go module tool will still look at the dependencies and
// add/keep them in go.mod so we can version these paths along with our other
// dependencies. When we run build on any of these paths, we get the version
// that has been specified in go.mod rather than the master copy.
