package scaffold

import (
	"fmt"
	"net/netip"
)

func parsePrefix(cidr string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("cidr %q is not a network: %w", cidr, err)
	}
	return prefix.Masked(), nil
}
