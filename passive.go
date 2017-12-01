package ftp

import (
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// openPassive creates a new passive data connection.
func (c *Client) openPassive() (*net.TCPConn, error) {
	addr, err := c.obtainPassiveAddress()
	if err != nil {
		return nil, err
	}
	return net.DialTCP("tcp", nil, addr)
}

// obtainPassiveAddress returns the address to dial
// for a new passive data connection.
func (c *Client) obtainPassiveAddress() (*net.TCPAddr, error) {
	if c.conn.RemoteAddr().Network() == "tcp6" {
		c.obtainPassiveAddress6()
	}
	return c.obtainPassiveAddress4()
}

func (c *Client) obtainPassiveAddress4() (*net.TCPAddr, error) {
	reply, err := c.sendCommand("PASV")
	if err != nil {
		return nil, err
	} else if reply.Code != CodePassive {
		return nil, reply
	}
	return parsePasvReply(reply.Msg)
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

func (c *Client) obtainPassiveAddress6() (*net.TCPAddr, error) {
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
