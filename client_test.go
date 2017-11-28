// Copyright (c) 2011 Ross Light.

package ftp

import (
	"bytes"
	"net"
	"net/textproto"
	"reflect"
	"testing"
)

type MockRWC struct {
	R, W *bytes.Buffer
}

func (conn MockRWC) Read(p []byte) (n int, err error) {
	return conn.R.Read(p)
}

func (conn MockRWC) Write(p []byte) (n int, err error) {
	return conn.W.Write(p)
}

func (conn MockRWC) Close() error {
	return nil
}

func TestClientResponse(t *testing.T) {
	tests := []struct {
		Input string
		Reply Reply
	}{
		{
			"201 Hello, World",
			Reply{201, "Hello, World"},
		},
		{
			"123-First line\r\nSecond line\r\n  234 A line beginning with numbers\r\n123 The last line",
			Reply{123, "First line\nSecond line\n  234 A line beginning with numbers\nThe last line"},
		},
	}
	for i, tt := range tests {
		client := &Client{
			proto: textproto.NewConn(MockRWC{
				R: bytes.NewBufferString(tt.Input),
				W: new(bytes.Buffer),
			}),
		}
		reply, err := client.response()
		if err != nil {
			t.Errorf("tests[%d] error: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.Reply, reply) {
			t.Errorf("tests[%d]: expected %#v (got %#v)", i, tt.Reply, reply)
		}
	}
}

func TestClientDo(t *testing.T) {
	const (
		expectedData = "NOOP\r\n"
		expectedCode = 500
		expectedMsg  = "Error"
	)

	rwc := MockRWC{
		R: bytes.NewBufferString("500 Error"),
		W: new(bytes.Buffer),
	}
	client := &Client{
		proto: textproto.NewConn(rwc),
	}
	reply, err := client.Do("NOOP")
	if err != nil {
		t.Fatal("error:", err)
	}
	if rwc.W.String() != expectedData {
		t.Errorf("Sent: %q (!= %q)", rwc.W.String(), expectedData)
	}
	if reply.Code != expectedCode {
		t.Errorf("Code: %v (!= %v)", reply.Code, expectedCode)
	}
	if reply.Msg != expectedMsg {
		t.Errorf("Msg: %v (!= %v)", reply.Msg, expectedMsg)
	}
}

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
