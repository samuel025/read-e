package main

import (
	"embed"
	_ "embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	// Tune glibc memory allocator to release freed heap pages back to OS aggressively
	_ = os.Setenv("MALLOC_TRIM_THRESHOLD_", "131072") // 128 KB
	_ = os.Setenv("MALLOC_ARENA_MAX", "2")

	// Encourage WebKitGTK to trim JavaScriptCore heap & cache on memory pressure
	if os.Getenv("WEBKIT_MEMORY_PRESSURE_RELIEF_PERCENT") == "" {
		_ = os.Setenv("WEBKIT_MEMORY_PRESSURE_RELIEF_PERCENT", "50")
	}

	// Disable compositing mode to avoid heavy GPU/Mesa driver allocations
	_ = os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "read e",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Linux: &linux.Options{
			Icon:             appIcon,
			ProgramName:      "read-e",
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
