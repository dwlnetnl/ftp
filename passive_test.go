// Copyright (c) 2011 Ross Light.
// Copyright (c) 2017, 2020 Anner van Hardenbroek.

package ftp

import (
	"net"
	"testing"
)

func TestParsePasvReply(t *testing.T) {
	var (
		expectedIP   = net.IPv4(192, 0, 2, 47)
		expectedPort = 1031
	)

	addr, err := parsePasvReply("227 Entering Passive Mode. 192,0,2,47,4,7")
	if err != nil {
		t.Fatal(err)
	}
	if !addr.IP.Equal(expectedIP) {
		t.Errorf("addr.IP = %v (expected %v)", addr.IP, expectedIP)
	}
	if addr.Port != expectedPort {
		t.Errorf("addr.Port = %v (expected %v)", addr.Port, expectedPort)
	}
}

func TestEpsvReply(t *testing.T) {
	const expectedPort = 1031
	port, err := parseEpsvReply("229 Entering Extended Passive Mode. (|||1031|)")
	if err != nil {
		t.Fatal(err)
	}
	if port != expectedPort {
		t.Errorf("port = %v (expected %v)", port, expectedPort)
	}
}
