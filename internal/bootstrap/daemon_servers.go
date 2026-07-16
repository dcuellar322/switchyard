package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"switchyard.dev/switchyard/internal/platform/localipc"
	"switchyard.dev/switchyard/internal/transport/httpapi"
)

type localServers struct {
	servers    []*http.Server
	listeners  []net.Listener
	ipcAddress string
}

func newLocalServers(config Config, dependencies httpapi.Dependencies) (*localServers, error) {
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
	return group, nil
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
