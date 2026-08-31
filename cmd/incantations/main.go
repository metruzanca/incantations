package main

import (
	"os"

	"github.com/metruzanca/incantations/internal/app"
	"github.com/metruzanca/incantations/internal/logutil"
)

func main() {
	logutil.Init()
	defer logutil.Close()
	os.Exit(app.New(os.Stdout, os.Stderr).Run(os.Args[1:]))
}
