package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

type queueListener struct {
	addr net.Addr
	ch   chan net.Conn
	done <-chan struct{}
}

func (l *queueListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.ch:
		if conn == nil {
			return nil, net.ErrClosed
		}
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *queueListener) Close() error   { return nil }
func (l *queueListener) Addr() net.Addr { return l.addr }

func (s *Server) currentTLSCertificate() (*tls.Certificate, error) {
	st := s.store.Snapshot()
	if strings.TrimSpace(st.Settings.CertFile) == "" || strings.TrimSpace(st.Settings.KeyFile) == "" {
		return nil, errors.New("HTTPS 证书路径未配置")
	}
	cert, err := tls.LoadX509KeyPair(st.Settings.CertFile, st.Settings.KeyFile)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func certificateValidForDomain(certFile, keyFile, domain string, now time.Time) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false
	}
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil || len(pair.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return false
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	intermediates := x509.NewCertPool()
	for _, der := range pair.Certificate[1:] {
		if cert, err := x509.ParseCertificate(der); err == nil {
			intermediates.AddCert(cert)
		}
	}
	_, err = leaf.Verify(x509.VerifyOptions{
		DNSName:       domain,
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   now,
	})
	return err == nil
}

func (s *Server) httpsReady() bool {
	st := s.store.Snapshot()
	return certificateValidForDomain(
		st.Settings.CertFile,
		st.Settings.KeyFile,
		st.Settings.Domain,
		time.Now(),
	)
}

func requestSecure(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer == nil || !(peer.IsLoopback() || peer.IsPrivate()) {
		return false
	}
	proto := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
	return proto == "https"
}

func panelPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err == nil && port != "" {
		return port
	}
	if i := strings.LastIndex(addr, ":"); i >= 0 && i+1 < len(addr) {
		if _, err := strconv.Atoi(addr[i+1:]); err == nil {
			return addr[i+1:]
		}
	}
	return "16800"
}

func (s *Server) redirectHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requestSecure(r) && s.httpsReady() {
			st := s.store.Snapshot()
			domain := strings.TrimSpace(st.Settings.Domain)
			port := panelPort(s.cfg.Addr)
			authority := domain
			if port != "443" {
				authority = net.JoinHostPort(domain, port)
			}
			target := "https://" + authority + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) servePanel(handler http.Handler) error {
	base, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	defer base.Close()

	done := make(chan struct{})
	httpQueue := &queueListener{addr: base.Addr(), ch: make(chan net.Conn, 64), done: done}
	httpsQueue := &queueListener{addr: base.Addr(), ch: make(chan net.Conn, 64), done: done}

	serverConfig := func() *http.Server {
		return &http.Server{
			Addr:              s.cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: 8 * time.Second,
			ReadTimeout:       20 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
	}

	httpServer := serverConfig()
	httpsServer := serverConfig()
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.currentTLSCertificate()
		},
	}

	errCh := make(chan error, 3)
	go func() { errCh <- httpServer.Serve(httpQueue) }()
	go func() { errCh <- httpsServer.Serve(tls.NewListener(httpsQueue, tlsConfig)) }()

	go func() {
		for {
			conn, err := base.Accept()
			if err != nil {
				errCh <- err
				return
			}
			go func(c net.Conn) {
				_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
				br := bufio.NewReader(c)
				first, err := br.Peek(1)
				if err != nil {
					_ = c.Close()
					return
				}
				_ = c.SetReadDeadline(time.Time{})
				wrapped := &bufferedConn{Conn: c, reader: br}
				target := httpQueue.ch
				if first[0] == 0x16 {
					target = httpsQueue.ch
				}
				select {
				case target <- wrapped:
				case <-done:
					_ = c.Close()
				}
			}(conn)
		}
	}()

	err = <-errCh
	close(done)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return fmt.Errorf("面板服务异常退出: %w", err)
}
