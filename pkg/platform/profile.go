package platform

// PlatformOS identifies the operating system.
type PlatformOS uint8

const (
	// OSUnknown is the zero value for an unrecognized OS.
	OSUnknown PlatformOS = iota
	OSWindows
	OSLinux
	OSMacOS
	OSAndroid
	OSIOS
	OSWeb
)

// String renders the OS name. Total switch.
func (o PlatformOS) String() string {
	switch o {
	case OSWindows:
		return "Windows"
	case OSLinux:
		return "Linux"
	case OSMacOS:
		return "macOS"
	case OSAndroid:
		return "Android"
	case OSIOS:
		return "iOS"
	case OSWeb:
		return "Web"
	default:
		return "Unknown"
	}
}

// PlatformArch identifies the CPU architecture / execution target.
type PlatformArch uint8

const (
	// ArchUnknown is the zero value for an unrecognized architecture.
	ArchUnknown PlatformArch = iota
	ArchAMD64
	ArchARM64
	ArchWASM
)

// String renders the architecture name. Total switch.
func (a PlatformArch) String() string {
	switch a {
	case ArchAMD64:
		return "amd64"
	case ArchARM64:
		return "arm64"
	case ArchWASM:
		return "wasm"
	default:
		return "unknown"
	}
}

// PlatformTier is the engine support commitment for a platform (L1 §4.1).
type PlatformTier uint8

const (
	// TierUnknown is the zero value.
	TierUnknown PlatformTier = iota
	// Tier1 — full support, CI every commit, release binaries (Windows/Linux/macOS).
	Tier1
	// Tier2 — supported, CI on release branches (Web/WASM, Android, iOS).
	Tier2
	// Tier3 — best-effort, no CI guarantee (Linux arm64, FreeBSD, consoles).
	Tier3
)

// String renders the tier. Total switch.
func (t PlatformTier) String() string {
	switch t {
	case Tier1:
		return "Tier1"
	case Tier2:
		return "Tier2"
	case Tier3:
		return "Tier3"
	default:
		return "TierUnknown"
	}
}

// PlatformProfile describes the current platform. It is inserted as an ECS
// resource during PreStartup and never mutated afterward (INV-2), so systems
// read it lock-free for runtime feature negotiation.
type PlatformProfile struct {
	OS           PlatformOS
	Arch         PlatformArch
	Tier         PlatformTier
	Capabilities PlatformCaps
}
