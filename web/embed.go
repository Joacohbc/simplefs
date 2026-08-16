package web

import (
	"embed"
)

// Assets contains all embedded web assets (templates and static files).
//
//go:embed templates/* static/*
var Assets embed.FS
