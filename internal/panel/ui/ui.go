package ui

import (
	"embed"
	"io/fs"
)

//go:embed web/build/*
var webAssets embed.FS

func GetFS() (fs.FS, error) {
	return fs.Sub(webAssets, "web/build")
}
