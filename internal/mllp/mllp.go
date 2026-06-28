// Package mllp implements the Minimal Lower Layer Protocol used to frame HL7
// v2 messages over TCP: <VT> message <FS><CR>. This is the transport OpenEMR
// core entirely lacks (it is file/SFTP/WS only), and the reason a daemon like
// this hub is needed.
package mllp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"
)

const (
	startBlock = 0x0b // VT
	endBlock   = 0x1c // FS
	carriage   = 0x0d // CR
)

// Handler processes one inbound message payload (HL7 sans framing) and returns
// the ACK payload to send back (also sans framing).
type Handler func(ctx context.Context, payload []byte, remote net.Addr) ([]byte, error)

// Server is an MLLP listener.
type Server struct {
	Addr    string
	Handler Handler
	Logger  *slog.Logger

	ln net.Listener
}

// ListenAndServe binds and serves until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("mllp listen %s: %w", s.Addr, err)
	}
	s.ln = ln
	s.log().Info("mllp server listening", "addr", ln.Addr().String())

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				s.log().Error("accept failed", "err", err)
				continue
			}
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		payload, err := readFrame(r)
		if err != nil {
			if err != io.EOF {
				s.log().Debug("connection closed", "remote", conn.RemoteAddr().String(), "err", err)
			}
			return
		}
		ack, herr := s.Handler(ctx, payload, conn.RemoteAddr())
		if herr != nil {
			s.log().Error("handler error", "err", herr)
			// Still attempt to return whatever ack the handler produced.
		}
		if len(ack) > 0 {
			if werr := writeFrame(conn, ack); werr != nil {
				s.log().Error("write ack failed", "err", werr)
				return
			}
		}
	}
}

func (s *Server) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// Send dials addr, sends one framed payload, and returns the framed ACK payload.
func Send(addr string, payload []byte, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("mllp dial %s: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if err := writeFrame(conn, payload); err != nil {
		return nil, err
	}
	return readFrame(bufio.NewReader(conn))
}

func writeFrame(w io.Writer, payload []byte) error {
	buf := make([]byte, 0, len(payload)+3)
	buf = append(buf, startBlock)
	buf = append(buf, payload...)
	buf = append(buf, endBlock, carriage)
	_, err := w.Write(buf)
	return err
}

// readFrame reads one MLLP frame, returning the unframed payload.
func readFrame(r *bufio.Reader) ([]byte, error) {
	// Discard bytes until the start block.
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == startBlock {
			break
		}
	}
	data, err := r.ReadBytes(endBlock)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSuffix(data, []byte{endBlock})
	// Consume the trailing CR after FS, if present.
	if cr, err := r.ReadByte(); err == nil && cr != carriage {
		_ = r.UnreadByte()
	}
	return data, nil
}
