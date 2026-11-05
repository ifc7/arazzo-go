package internal

import (
	"runtime/debug"
)

// Build metadata injected via ldflags.
var (
	BuildVersion = "dev"
	GitCommit    = "unknown"
	BuildTime    = "unknown"
)

func init() {
	fillFromBuildInfo()
}

// fillFromBuildInfo populates any build metadata not set via ldflags from the
// information the Go toolchain embeds in the binary. This covers
// `go install github.com/ifc7/arazzo-go/cmd/arazzo@vX.Y.Z` (module version)
// and plain `go build` inside a git checkout (VCS revision and time).
func fillFromBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if BuildVersion == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		BuildVersion = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if GitCommit == "unknown" && s.Value != "" {
				GitCommit = s.Value
				if len(GitCommit) > 12 {
					GitCommit = GitCommit[:12]
				}
			}
		case "vcs.time":
			if BuildTime == "unknown" && s.Value != "" {
				BuildTime = s.Value
			}
		}
	}
}
