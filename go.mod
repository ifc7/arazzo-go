module github.com/shaunhoulihan/arazzo-go

go 1.25.7

require (
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/mattn/go-isatty v0.0.24
	github.com/pb33f/libopenapi v0.38.7
	github.com/spf13/cobra v1.10.2
	go.yaml.in/yaml/v4 v4.0.0-rc.6
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/pb33f/jsonpath v0.8.3 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

// ifc7/libopenapi: &&/||/parentheses in simple criteria, nil for missing
// step outputs / empty response bodies, and integer 0/1 in simple conditions
// (strconv.ParseBool must not win over ParseInt).
replace github.com/pb33f/libopenapi => github.com/ifc7/libopenapi v0.0.0-20260904033543-203ee234ac75
