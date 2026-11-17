package config

import (
	"fmt"
	"net/netip"
	"time"
)

// parseCIDR reads an IPv4 network and returns it with the host bits cleared.
func parseCIDR(s string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("only IPv4 networks are supported")
	}
	return prefix.Masked(), nil
}

// parseIP reads one IPv4 address.
func parseIP(s string) (netip.Addr, error) {
	address, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, err
	}
	if !address.Is4() {
		return netip.Addr{}, fmt.Errorf("only IPv4 addresses are supported")
	}
	return address, nil
}

// prefixLast returns the last address in a network.
func prefixLast(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Masked().Addr().As4()
	bits := prefix.Bits()
	for i := 0; i < 4; i++ {
		for bit := 0; bit < 8; bit++ {
			position := i*8 + bit
			if position < bits {
				continue
			}
			bytes[i] |= 1 << (7 - bit)
		}
	}
	return netip.AddrFrom4(bytes)
}

func mustDuration(s string) time.Duration {
	parsed, err := ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return parsed
}
