package main

import (
	"os"

	"golang.org/x/term"

	"github.com/metruzanca/incantations/internal/app"
	"github.com/metruzanca/incantations/internal/logutil"
	"github.com/metruzanca/incantations/internal/ui"
)

func main() {
	logutil.Init()
	defer logutil.Close()
	ui.Styled = term.IsTerminal(int(os.Stdout.Fd()))
	os.Exit(app.New(os.Stdout, os.Stderr).Run(os.Args[1:]))
}
