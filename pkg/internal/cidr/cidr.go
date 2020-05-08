package cidr

import (
	"encoding/binary"
	"math"
	"net"

	"github.com/pkg/errors"
)

// SplitIntoSubnets splits a CIDR into a specified number of subnets.
// If the number of required subnets isn't a power of 2 then then CIDR will be split
// into the the next highest power of 2 and you will end up with unused ranges.
// NOTE: this code is adapted from kops https://github.com/kubernetes/kops/blob/c323819e6480d71bad8d21184516e3162eaeca8f/pkg/util/subnet/subnet.go#L46
func SplitIntoSubnets(cidrBlock string, numSubnets int) ([]*net.IPNet, error) {
	_, parent, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse CIDR")
	}

	subnetBits := math.Ceil(math.Log2(float64(numSubnets)))

	networkLen, addrLen := parent.Mask.Size()
	modifiedNetworkLen := networkLen + int(subnetBits)

	if modifiedNetworkLen > addrLen {
		return nil, errors.Errorf("cidr %s cannot accomodate %d subnets", cidrBlock, numSubnets)
	}

	var subnets []*net.IPNet
	for i := 0; i < numSubnets; i++ {
		ip4 := parent.IP.To4()
		if ip4 == nil {
			return nil, errors.Errorf("unexpected IP address type: %s", parent)
		}

		n := binary.BigEndian.Uint32(ip4)
		n += uint32(i) << uint(32-modifiedNetworkLen)
		subnetIP := make(net.IP, len(ip4))
		binary.BigEndian.PutUint32(subnetIP, n)

		subnets = append(subnets, &net.IPNet{
			IP:   subnetIP,
			Mask: net.CIDRMask(modifiedNetworkLen, 32),
		})
	}

	return subnets, nil
}
