// Copyright (c) 2011 Ross Light.
// Copyright (c) 2017 Anner van Hardenbroek.

// Package ftp provides a minimal FTP client as defined in RFC 959.
package ftp

import (
	"errors"
	"io"
	"net"
	"net/textproto"
	"regexp"
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

// obtainPassiveAddress returns the address to dial for a new passive data
// connection.
func (c *Client) obtainPassiveAddress() (*net.TCPAddr, error) {
	if c.conn.RemoteAddr().Network() == "tcp6" {
		reply, err := c.sendCommand("EPSV")
		if err != nil {
			return nil, err
		} else if reply.Code != CodeExtendedPassive {
			return nil, reply
		}

		port, err := parseEpsvReply(reply.Msg)
		if err != nil {
			return nil, err
		}

		return &net.TCPAddr{
			IP:   c.conn.RemoteAddr().(*net.TCPAddr).IP,
			Port: port,
		}, nil
	}

	reply, err := c.sendCommand("PASV")
	if err != nil {
		return nil, err
	} else if reply.Code != CodePassive {
		return nil, reply
	}
	return parsePasvReply(reply.Msg)
}

// openPassive creates a new passive data connection.
func (c *Client) openPassive() (*net.TCPConn, error) {
	addr, err := c.obtainPassiveAddress()
	if err != nil {
		return nil, err
	}
	return net.DialTCP("tcp", nil, addr)
}

var pasvRegexp = regexp.MustCompile(`([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+),([0-9]+)`)

func parsePasvReply(msg string) (*net.TCPAddr, error) {
	numberStrings := pasvRegexp.FindStringSubmatch(msg)
	if numberStrings == nil {
		return nil, errors.New("PASV reply provided no port")
	}
	numbers := make([]byte, len(numberStrings))
	for i, s := range numberStrings {
		n, _ := strconv.Atoi(s)
		numbers[i] = byte(n)
	}
	return &net.TCPAddr{
		IP:   net.IP(numbers[1:5]),
		Port: int(numbers[5])<<8 | int(numbers[6]),
	}, nil
}

const (
	epsvStart = "(|||"
	epsvEnd   = "|)"
)

func parseEpsvReply(msg string) (port int, err error) {
	start := strings.LastIndex(msg, epsvStart)
	if start == -1 {
		return 0, errors.New("EPSV reply provided no port")
	}
	start += len(epsvStart)

	end := strings.LastIndex(msg, epsvEnd)
	if end == -1 || end <= start {
		return 0, errors.New("EPSV reply provided no port")
	}

	return strconv.Atoi(msg[start:end])
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
