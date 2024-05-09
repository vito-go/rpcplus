package gorpc

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
)

type CodecClient interface {
	WriteRequest(r *Request, args ...interface{}) error
	ReadResponseHeader() (r *Response, err error)
	ReadResponseBody(any) (err error)
	Close() error
}

type CodecServer interface {
	ReadRequestHeader() (*Request, error)
	ReadRequestBody(...any) error
	WriteResponse(r *Response, body any) (err error)
	Close() error
}
type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

var _ CodecServer = (*gobServerCodec)(nil)

func (c *gobServerCodec) ReadRequestBody(args ...any) error {
	var stopErr error
	for i := 0; i < len(args); i++ {
		arg := args[i]
		err := c.dec.Decode(arg)
		if err != nil {
			stopErr = err
			continue
		}
	}
	if stopErr != nil {
		return stopErr
	}
	return nil
}

func newGobServerCodec(conn net.Conn) *gobServerCodec {
	buf := bufio.NewWriter(conn)
	return &gobServerCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(buf),
		encBuf: buf,
	}
}

func (c *gobServerCodec) ReadRequestHeader() (*Request, error) {
	var req Request
	var err error
	const maxArgAum = 64
	if err = c.dec.Decode(&req); err != nil {
		return nil, err
	}
	if req.ArgsNum == 0 {
		return &req, nil
	}
	if req.ArgsNum > maxArgAum {
		return nil, fmt.Errorf("too many arguments")
	}
	return &req, nil
}

func (c *gobServerCodec) WriteResponse(r *Response, body any) (err error) {
	if err = c.enc.Encode(r); err != nil {
		if c.encBuf.Flush() == nil {
			// Gob couldn't encode the Response. Should not happen, so if it does,
			// shut down the connection to signal that the connection is broken.
			log.Println("gorpc: gob error encoding response:", err)
			c.Close()
		}
		return
	}
	err = c.encBuf.Flush()
	if err != nil {
		return err
	}
	if err = c.enc.Encode(body); err != nil {
		if c.encBuf.Flush() == nil {
			// Was a gob problem encoding the body but the header has been written.
			// Shut down the connection to signal that the connection is broken.
			log.Println("rpc: gob error encoding body:", err)
			c.Close()
		}
		return
	}

	return c.encBuf.Flush()
}

func (c *gobServerCodec) Close() error {
	if c.closed {
		// Only call c.rwc.Close once; otherwise the semantics are undefined.
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}

var _ CodecClient = (*gobClientCodec)(nil)

type gobClientCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
}

func newGobClientCodec(conn net.Conn) *gobClientCodec {
	encBuf := bufio.NewWriter(conn)
	return &gobClientCodec{conn, gob.NewDecoder(conn), gob.NewEncoder(encBuf), encBuf}
}
func (g *gobClientCodec) WriteRequest(request *Request, args ...interface{}) error {
	var err error
	if err = g.enc.Encode(request); err != nil {
		return err
	}
	if err = g.encBuf.Flush(); err != nil {
		return err
	}

	for _, arg := range args {
		if err = g.enc.Encode(arg); err != nil {
			return err
		}
		if err = g.encBuf.Flush(); err != nil {
			return err
		}
	}
	return g.encBuf.Flush()
}

func (g *gobClientCodec) ReadResponseHeader() (r *Response, err error) {
	var resp Response
	if err = g.dec.Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
func (g *gobClientCodec) ReadResponseBody(body any) (err error) {
	if err = g.dec.Decode(body); err != nil {
		return err
	}
	return nil
}

func (g *gobClientCodec) Close() error {
	return g.rwc.Close()
}
