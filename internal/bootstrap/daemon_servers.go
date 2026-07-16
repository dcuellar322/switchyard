package bootstrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"switchyard.dev/switchyard/internal/platform/localipc"
	"switchyard.dev/switchyard/internal/transport/httpapi"
)

type localServers struct {
	servers       []*http.Server
	listeners     []net.Listener
	ipcAddress    string
	remoteAddress string
}

func newLocalServers(config Config, dependencies httpapi.Dependencies, routeHandler, remoteHandler http.Handler) (*localServers, error) {
	browserListener, err := net.Listen("tcp", config.HTTPAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on loopback API: %w", err)
	}
	group := &localServers{
		servers:    []*http.Server{newHTTPServer(httpapi.NewBrowser(dependencies))},
		listeners:  []net.Listener{browserListener},
		ipcAddress: config.IPCAddr,
	}
	if group.ipcAddress == "" {
		group.ipcAddress = localipc.DefaultAddress(config.DataDir)
	}
	ipcListener, err := localipc.Listener(group.ipcAddress)
	if err != nil && !errors.Is(err, localipc.ErrUnsupported) {
		_ = browserListener.Close()
		return nil, err
	}
	if ipcListener != nil {
		group.servers = append(group.servers, newHTTPServer(httpapi.NewIPC(dependencies)))
		group.listeners = append(group.listeners, ipcListener)
	}
	if config.RoutingAddr != "" {
		if routeHandler == nil {
			_ = browserListener.Close()
			if ipcListener != nil {
				_ = ipcListener.Close()
			}
			return nil, errors.New("local route listener requires a proxy handler")
		}
		routeListener, listenErr := net.Listen("tcp", config.RoutingAddr)
		if listenErr != nil {
			for _, listener := range group.listeners {
				_ = listener.Close()
			}
			return nil, fmt.Errorf("listen for local routes: %w", listenErr)
		}
		group.servers = append(group.servers, newHTTPServer(routeHandler))
		group.listeners = append(group.listeners, routeListener)
	}
	if config.RemoteAddr != "" {
		if remoteHandler == nil {
			for _, listener := range group.listeners {
				_ = listener.Close()
			}
			return nil, errors.New("remote listener requires an authenticated agent handler")
		}
		listener, listenErr := newRemoteListener(config)
		if listenErr != nil {
			for _, current := range group.listeners {
				_ = current.Close()
			}
			return nil, listenErr
		}
		group.servers = append(group.servers, newHTTPServer(remoteHandler))
		group.listeners = append(group.listeners, listener)
		group.remoteAddress = listener.Addr().String()
	}
	return group, nil
}

func newRemoteListener(config Config) (net.Listener, error) {
	certificate, err := tls.LoadX509KeyPair(config.RemoteTLSCertificate, config.RemoteTLSKey)
	if err != nil {
		return nil, fmt.Errorf("load remote server identity: %w", err)
	}
	caDocument, err := os.ReadFile(config.RemoteClientCA)
	if err != nil {
		return nil, fmt.Errorf("read remote client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caDocument) {
		return nil, errors.New("remote client CA is invalid")
	}
	listener, err := net.Listen("tcp", config.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("listen for authenticated remote agents: %w", err)
	}
	return tls.NewListener(listener, &tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientCAs: clientCAs, ClientAuth: tls.RequireAndVerifyClientCert,
	}), nil
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 2 * time.Minute}
}

func (s *localServers) browserAddress() string { return s.listeners[0].Addr().String() }

func (s *localServers) run(ctx context.Context, shutdownOperations func(context.Context) error) error {
	errorsChannel := make(chan error, len(s.servers))
	for index, server := range s.servers {
		server, listener := server, s.listeners[index]
		go func() { errorsChannel <- server.Serve(listener) }()
	}
	select {
	case <-ctx.Done():
		return s.shutdown(errorsChannel, shutdownOperations)
	case serveErr := <-errorsChannel:
		for _, server := range s.servers {
			_ = server.Close()
		}
		for index := 1; index < len(s.servers); index++ {
			<-errorsChannel
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serveErr)
		}
		return nil
	}
}

func (s *localServers) shutdown(errorsChannel <-chan error, shutdownOperations func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, server := range s.servers {
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
	}
	if err := shutdownOperations(ctx); err != nil {
		return err
	}
	for range s.servers {
		serveErr := <-errorsChannel
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serveErr)
		}
	}
	return nil
}
