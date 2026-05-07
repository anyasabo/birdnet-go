//go:build !skipfrontend

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distDir embed.FS

// DistFS is the embedded Svelte build output filesystem
var DistFS fs.FS

func init() {
	var err error
	DistFS, err = fs.Sub(distDir, "dist")
	if err != nil {
		panic("frontend: failed to initialize embedded filesystem: " + err.Error())
	}
}
