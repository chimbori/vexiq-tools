package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"vexiqtools/ui"
)

//go:embed static
var staticFiles embed.FS

func main() {
	var app *application.App

	pickDirectory := func() (string, error) {
		return app.Dialog.OpenFile().
			SetTitle("Choose Directory").
			CanChooseDirectories(true).
			CanChooseFiles(false).
			PromptForSingleSelection()
	}

	settingsPath, err := ui.DefaultSettingsPath()
	if err != nil {
		log.Printf("settings are not saved: %v", err)
	}

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	app = application.New(application.Options{
		Name:        "Tools for Vex IQ",
		Description: "Tools for VEX IQ developers",
		Assets: application.AssetOptions{
			Handler: ui.NewHandler(static, pickDirectory, settingsPath),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Tools for Vex IQ",
		Width:  800,
		Height: 600,
		URL:    "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
