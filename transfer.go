package ftp

import (
	"context"
	"io"
)

// Text sends a command and opens a new passive data connection in ASCII mode.
func (c *Client) Text(ctx context.Context, command string) (io.ReadWriteCloser, error) {
	return c.transfer(ctx, command, "A")
}

// Binary sends a command and opens a new passive data connection in image mode.
func (c *Client) Binary(ctx context.Context, command string) (io.ReadWriteCloser, error) {
	return c.transfer(ctx, command, "I")
}

// transfer sends a command and opens a new passive data connection.
func (c *Client) transfer(ctx context.Context, command, dataType string) (io.ReadWriteCloser, error) {
	// Set type
	if reply, err := c.sendCommand(ctx, "TYPE "+dataType); err != nil {
		return nil, err
	} else if !reply.PositiveComplete() {
		return nil, reply
	}

	// Open data connection
	conn, err := c.openPassive(ctx)
	if err != nil {
		return nil, err
	}
	defer func(conn io.Closer) {
		if err != nil {
			conn.Close()
		}
	}(conn)

	// Send command
	if reply, err := c.sendCommand(ctx, command); err != nil {
		return nil, err
	} else if !reply.Positive() {
		return nil, reply
	}
	return &transferConn{conn, c, ctx}, nil
}

type transferConn struct {
	io.ReadWriteCloser
	c   *Client
	ctx context.Context
}

func (tc *transferConn) Close() error {
	if tc.ctx.Done() == nil {
		return tc.close()
	}
	ch := make(chan error, 1)
	go func() {
		ch <- tc.close()
	}()
	select {
	case err := <-ch:
		return err
	case <-tc.ctx.Done():
		// close tc to read the response
		// on the main connection (client)
		tc.close()
		return tc.ctx.Err()
	}
}

func (tc *transferConn) close() error {
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
