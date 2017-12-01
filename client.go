// Copyright (c) 2011 Ross Light.
// Copyright (c) 2017 Anner van Hardenbroek.

// Package ftp provides a minimal FTP client as defined in RFC 959.
package ftp

import (
	"errors"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
)

// A Client is an FTP client.
// A single FTP connection cannot handle simultaneous transfers.
type Client struct {
	conn    net.Conn
	proto   *textproto.Conn
	Welcome Reply
}

// Dial connects to an FTP server.
func Dial(network, addr string) (*Client, error) {
	c, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	return NewClient(c)
}

// NewClient creates an FTP client from an existing connection.
func NewClient(conn net.Conn) (*Client, error) {
	var err error
	c := &Client{
		conn:  conn,
		proto: textproto.NewConn(conn),
	}
	c.Welcome, err = c.response()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Quit sends the QUIT command and closes the connection.
func (c *Client) Quit() error {
	if _, err := c.sendCommand("QUIT"); err != nil {
		return err
	}
	return c.Close()
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.proto.Close()
}

// Login sends credentials to the server.
func (c *Client) Login(username, password string) error {
	reply, err := c.sendCommand("USER " + username)
	if err != nil {
		return err
	}
	if reply.Code == CodeNeedPassword {
		reply, err = c.sendCommand("PASS " + password)
		if err != nil {
			return err
		}
	}
	if !reply.PositiveComplete() {
		return reply
	}
	return nil
}

// Do sends a command over the control connection and waits for the response.
// It returns any protocol error encountered while performing the command.
func (c *Client) Do(command string) (Reply, error) {
	return c.sendCommand(command)
}

type transferConn struct {
	io.ReadWriteCloser
	c *Client
}

func (tc transferConn) Close() error {
	if err := tc.ReadWriteCloser.Close(); err != nil {
		return err
	}

	if reply, err := tc.c.response(); err != nil {
		return err
	} else if !reply.PositiveComplete() {
		return reply
	}
	return nil
}

// transfer sends a command and opens a new passive data connection.
func (c *Client) transfer(command, dataType string) (conn io.ReadWriteCloser, err error) {
	// Set type
	if reply, err := c.sendCommand("TYPE " + dataType); err != nil {
		return nil, err
	} else if !reply.PositiveComplete() {
		return nil, reply
	}

	// Open data connection
	conn, err = c.openPassive()
	if err != nil {
		return nil, err
	}
	defer func(conn io.Closer) {
		if err != nil {
			conn.Close()
		}
	}(conn)

	// Send command
	if reply, err := c.sendCommand(command); err != nil {
		return nil, err
	} else if !reply.Positive() {
		return nil, reply
	}
	return transferConn{conn, c}, nil
}

// Text sends a command and opens a new passive data connection in ASCII mode.
func (c *Client) Text(command string) (io.ReadWriteCloser, error) {
	return c.transfer(command, "A")
}

// Binary sends a command and opens a new passive data connection in image mode.
func (c *Client) Binary(command string) (io.ReadWriteCloser, error) {
	return c.transfer(command, "I")
}

func (c *Client) sendCommand(command string) (Reply, error) {
	err := c.proto.PrintfLine("%s", command)
	if err != nil {
		return Reply{}, err
	}
	return c.response()
}

// response reads a reply from the server.
func (c *Client) response() (Reply, error) {
	line, err := c.proto.ReadLine()
	if err != nil {
		return Reply{}, err
	} else if len(line) < 4 {
		return Reply{}, errors.New("Short response line in FTP")
	}

	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return Reply{}, err
	}

	reply := Reply{Code: Code(code)}
	switch line[3] {
	case '-':
		lines := []string{line[4:]}
		endPrefix := strconv.Itoa(code) + " "
		for {
			line, err = c.proto.ReadLine()
			if err != nil {
				break
			}
			if strings.HasPrefix(line, endPrefix) {
				lines = append(lines, line[len(endPrefix):])
				break
			} else {
				lines = append(lines, line)
			}
		}
		reply.Msg = strings.Join(lines, "\n")
		return reply, err
	case ' ':
		reply.Msg = line[4:]
	default:
		return Reply{}, errors.New("Expected space after FTP response code")
	}
	return reply, nil
}
