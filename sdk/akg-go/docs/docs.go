// Package docs embeds the akg-go documentation graph for use by the CLI.
package docs

import _ "embed"

//go:embed akg-go-docs.akg
var Graph []byte
