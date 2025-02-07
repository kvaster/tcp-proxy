package proxy

import (
	"fmt"
	"github.com/apex/log"
	"io"
	"net"
	"syscall"
)

const SoOriginalDst = 80

type Server struct {
	listenAddr string
	mark       int

	connListener net.Listener
}

func New(listenAddr string, mark int) *Server {
	return &Server{
		listenAddr: listenAddr,
		mark:       mark,
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	s.connListener = ln

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Debug("finished accepting new connections")
				break
			}

			go s.handleConnection(conn)
		}
	}()

	return nil
}

func (s *Server) Stop() {
	_ = s.connListener.Close()
	// TODO: implement gracefully shutdown of active connections with context
	log.Info("stopped")
}

func (s *Server) handleConnection(conn net.Conn) {
	l := log.WithField("src", conn.RemoteAddr().String())

	l.Info("connection accepted")

	defer func() {
		_ = conn.Close()
	}()

	tcpConn := conn.(*net.TCPConn)

	file, err := tcpConn.File()
	if err != nil {
		l.WithError(err).Error("failed to get file descriptor")
		return
	}
	defer func() {
		_ = file.Close()
	}()

	var destAddr string

	fd := int(file.Fd())
	addr, err := syscall.GetsockoptIPv6Mreq(fd, syscall.IPPROTO_IP, SoOriginalDst)
	if err != nil {
		l.WithError(err).Error("failed to get original destination")
		return
	} else {
		// parse only ipv4 for now
		dstIP := net.IPv4(addr.Multiaddr[4], addr.Multiaddr[5], addr.Multiaddr[6], addr.Multiaddr[7])
		dstPort := uint16(addr.Multiaddr[2])<<8 + uint16(addr.Multiaddr[3])
		destAddr = fmt.Sprintf("%s:%d", dstIP.String(), dstPort)

		localAddr := conn.LocalAddr().String()
		if destAddr == localAddr {
			l.WithField("local", localAddr).WithField("dst", destAddr).Error("destination address is the same as local address")
			return
		}
	}

	l = l.WithField("dst", destAddr)
	l.Debug("dialing")

	d := net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, s.mark)
				if err != nil {
					l.WithError(err).Error("failed to set socket mark")
				}
			})
		},
	}

	serverConn, err := d.Dial("tcp", destAddr)
	if err != nil {
		l.WithError(err).Error("dial failed")
		return
	}
	defer func() {
		_ = serverConn.Close()
	}()

	l.Debug("serving")

	go func() {
		_, err := io.Copy(serverConn, conn)
		if err != nil && err != io.EOF {
			l.Errorf("error copying client -> server: %v", err)
		}
	}()

	_, err = io.Copy(conn, serverConn)
	if err != nil && err != io.EOF {
		l.Errorf("error copying server -> client: %v", err)
	}

	l.Info("connection closed")
}
