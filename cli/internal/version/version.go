package version

// Version is the current version of the WarpDrop CLI.
// This value can be overridden at build time using:
//   go build -ldflags="-X 'github.com/BioHazard786/Warpdrop/cli/internal/version.Version=v1.0.0'"
// GoReleaser will automatically set this during release builds.
var Version = "dev"
