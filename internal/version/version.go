package version

// Version is set at build time via:
//
//	-ldflags "-X github.com/yegor-usoltsev/cronctl/internal/version.Version=v1.2.3"
//
// When not set, it defaults to "dev".
var Version = "dev" //nolint:gochecknoglobals
