// Copyright (c) 2011 Ross Light.
// Copyright (c) 2017 Anner van Hardenbroek.

// Package ftp provides a minimal FTP client as defined in RFC 959.
package ftp

import (
	"context"
	"errors"
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

// Dial connects to an FTP server using the provided context.
func Dial(ctx context.Context, network, addr string) (*Client, error) {
	if !strings.HasPrefix(network, "tcp") {
		return nil, errors.New("ftp: only TCP connections are supported")
	}
	var d net.Dialer
	c, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	return NewClient(ctx, c)
}

// NewClient creates an FTP client from an existing connection.
// It reads the initial (welcome) message from the server.
func NewClient(ctx context.Context, conn net.Conn) (*Client, error) {
	var err error
	c := &Client{
		conn:  conn,
		proto: textproto.NewConn(conn),
	}
	c.Welcome, err = c.readWelcome(ctx)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) readWelcome(ctx context.Context) (Reply, error) {
	if ctx.Done() == nil {
		return c.response()
	}
	resp := make(chan response, 1)
	go func() {
		r, err := c.response()
		resp <- response{r, err}
	}()
	select {
	case r := <-resp:
		return r.reply, r.err
	case <-ctx.Done():
		return Reply{}, ctx.Err()
	}
}

// Quit sends the QUIT command and closes the connection.
func (c *Client) Quit(ctx context.Context) error {
	_, err := c.sendCommand(ctx, "QUIT")
	if err == context.Canceled || err == context.DeadlineExceeded {
		return c.Close()
	}
	if err != nil {
		return err
	}
	return c.Close()
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.proto.Close()
}

// Login sends credentials to the server.
func (c *Client) Login(ctx context.Context, username, password string) error {
	reply, err := c.sendCommand(ctx, "USER "+username)
	if err != nil {
		return err
	}
	if reply.Code == CodeNeedPassword {
		reply, err = c.sendCommand(ctx, "PASS "+password)
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
func (c *Client) Do(ctx context.Context, command string) (Reply, error) {
	return c.sendCommand(ctx, command)
}

func (c *Client) sendCommand(ctx context.Context, command string) (Reply, error) {
	if ctx.Done() == nil {
		r := c.sendCmd(command)
		return r.reply, r.err
	}
	result := make(chan response)
	go c.sendCmd(command)
	select {
	case r := <-result:
		return r.reply, r.err
	case <-ctx.Done():
		return Reply{}, ctx.Err()
	}
}

type response struct {
	reply Reply
	err   error
}

func (c *Client) sendCmd(command string) response {
	err := c.proto.PrintfLine("%s", command)
	if err != nil {
		return response{err: err}
	}
	r, err := c.response()
	return response{r, err}
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
