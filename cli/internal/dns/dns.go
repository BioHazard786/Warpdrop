package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// PublicDNS are servers to be queried if a local lookup fails
// These are well-known, high-availability public DNS providers
var publicDNS = []string{
	"1.0.0.1",                // Cloudflare
	"1.1.1.1",                // Cloudflare
	"[2606:4700:4700::1111]", // Cloudflare
	"[2606:4700:4700::1001]", // Cloudflare
	"8.8.4.4",                // Google
	"8.8.8.8",                // Google
	"[2001:4860:4860::8844]", // Google
	"[2001:4860:4860::8888]", // Google
	"9.9.9.9",                // Quad9
	"149.112.112.112",        // Quad9
	"[2620:fe::fe]",          // Quad9
	"[2620:fe::fe:9]",        // Quad9
	"8.26.56.26",             // Comodo
	"8.20.247.20",            // Comodo
	"208.67.220.220",         // Cisco OpenDNS
	"208.67.222.222",         // Cisco OpenDNS
	"[2620:119:35::35]",      // Cisco OpenDNS
	"[2620:119:53::53]",      // Cisco OpenDNS
}

// Lookup resolves a hostname to an IP address.
// It first attempts to use the system's default resolver.
// If that fails, it falls back to using public DNS providers directly.
func Lookup(address string) (string, error) {
	// 1. Try Local/System DNS first
	ip, err := localLookupIP(address)
	if err == nil && ip != "" {
		return ip, nil
	}

	// 2. Fallback to Internal/Public DNS
	// ui.PrintWarning(fmt.Sprintf("System DNS lookup failed for %s, falling back to public DNS...", address))
	return remoteLookupWithRace(address)
}

// localLookupIP returns a host's IP address using the local DNS configuration.
func localLookupIP(address string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	r := &net.Resolver{}
	ips, err := r.LookupHost(ctx, address)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", errors.New("no IP addresses found")
	}

	// Prefer IPv4
	for _, ip := range ips {
		if net.ParseIP(ip).To4() != nil {
			return ip, nil
		}
	}

	return ips[0], nil
}

// remoteLookupWithRace returns a host's IP address by racing multiple public DNS servers.
func remoteLookupWithRace(address string) (string, error) {
	// Create a buffered channel to receive the first successful result
	type result struct {
		ip  string
		err error
	}

	// We'll limit concurrency to avoid spamming too many connections if list grows,
	// but for ~8 servers, doing them all at once is fine for speed.
	results := make(chan result, len(publicDNS))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for _, dnsServer := range publicDNS {
		go func(server string) {
			ip, err := remoteLookupIP(ctx, address, server)
			results <- result{ip: ip, err: err}
		}(dnsServer)
	}

	// Wait for the first success or all failures
	failureCount := 0
	for range publicDNS {
		select {
		case res := <-results:
			if res.err == nil && res.ip != "" {
				return res.ip, nil
			}
			failureCount++
		case <-ctx.Done():
			return "", fmt.Errorf("DNS lookup timed out during public DNS race")
		}
	}

	return "", fmt.Errorf("failed to resolve %s: all %d public DNS servers failed or exhausted", address, failureCount)
}

// remoteLookupIP queries a specific DNS server for the address.
func remoteLookupIP(ctx context.Context, address, dnsServer string) (string, error) {
	// Use a custom dialer to force connection to the specific DNS server
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := new(net.Dialer)
			// Force port 53 for DNS
			return d.DialContext(ctx, network, net.JoinHostPort(dnsServer, "53"))
		},
	}

	ips, err := r.LookupHost(ctx, address)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", errors.New("no IPs returned")
	}

	// Prefer IPv4
	for _, ip := range ips {
		if net.ParseIP(ip).To4() != nil {
			return ip, nil
		}
	}

	return ips[0], nil
}
