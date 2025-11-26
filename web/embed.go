// web/embed.go
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func init() {
	// Debug: list embedded files at startup
	fs.WalkDir(staticFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fmt.Printf("Embedded: %s\n", path)
		return nil
	})
}

func StaticFS() http.FileSystem {
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}

func GetFile(name string) ([]byte, error) {
	path := "static/" + name
	content, err := staticFiles.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read embedded file: %s, error: %v\n", path, err)
	}
	return content, err
}
