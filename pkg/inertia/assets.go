package inertia

import "embed"

//go:embed all:public
var PublicFS embed.FS

//go:embed all:resources
var ResourceFS embed.FS
