package netutil

import (
	"context"
	"fmt"
	"net"
)

func ResolveIPv4(ctx context.Context, host string) (string, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IPv4 address found for %q", host)
	}
	return ips[0].String(), nil
}
