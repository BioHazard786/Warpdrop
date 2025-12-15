package utils

import (
	"net"
	"strings"
)

// ShouldForceRelay checks if the system is likely behind a restrictive VPN or CGNAT
// and returns true if we should force TURN usage.
func ShouldForceRelay() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	// 1. Define the CGNAT range (100.64.0.0/10)
	// Cloudflare WARP, Tailscale, and Carrier Grade NATs use this.
	// If we are on one of these, direct P2P often fails or requires relay anyway.
	_, cgnatBlock, _ := net.ParseCIDR("100.64.0.0/10")

	for _, iface := range interfaces {
		// Ignore loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 2. Check Interface Name Heuristics
		name := strings.ToLower(iface.Name)
		if strings.Contains(name, "tun") || // Standard VPNs (OpenVPN, etc)
			strings.Contains(name, "tap") || // Virtual adapters
			strings.Contains(name, "wg") || // WireGuard
			strings.Contains(name, "ppp") || // Point-to-Point
			strings.Contains(name, "warp") { // Explicit WARP
			return true
		}

		// 3. Check IP Addresses for CGNAT
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// If we find an IP inside 100.64.0.0/10, we are likely behind WARP or CGNAT
			if cgnatBlock.Contains(ip) {
				return true
			}
		}
	}

	return false
}
