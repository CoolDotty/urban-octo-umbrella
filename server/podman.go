package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

var errPodmanUnavailable = errors.New("podman not available")

const (
	podmanUnavailableMessage = "Podman is not available on the server."
	podmanLoadFailedMessage  = "Failed to load Podman containers."
)

const (
	podmanEventRestartDelay = 2 * time.Second
	podmanPollDebounce      = 1 * time.Second
	podmanRemoveDebounce    = 2 * time.Second
	podmanClientBufferSize  = 16
	podmanWriteTimeout      = 5 * time.Second
)

type podmanContainer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Status      string            `json:"status"`
	StorageSize string            `json:"storageSize,omitempty"`
	CreatedAt   string            `json:"createdAt,omitempty"`
	Ports       string            `json:"ports,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type podmanStreamMessage struct {
	Type    string            `json:"type"`
	Data    []podmanContainer `json:"data"`
	Message string            `json:"message,omitempty"`
}

type podmanEvent struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	Image  string `json:"Image"`
	Status string `json:"Status"`
	Type   string `json:"Type"`
}

type podmanService struct {
	mu          sync.RWMutex
	containers  []podmanContainer
	hash        uint64
	errMessage  string
	initialized bool

	hubMu   sync.Mutex
	clients map[*podmanClient]struct{}

	pollCh chan time.Duration
	once   sync.Once
}

type podmanClient struct {
	conn      *websocket.Conn
	sendCh    chan podmanStreamMessage
	closeCh   chan struct{}
	closeOnce sync.Once
}

func newPodmanService() *podmanService {
	return &podmanService{
		containers: []podmanContainer{},
		clients:    make(map[*podmanClient]struct{}),
		pollCh:     make(chan time.Duration, 1),
	}
}

func (s *podmanService) start(app core.App) {
	s.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
			cancel()
			return e.Next()
		})

		go s.runPoller(ctx)
		go s.runEventListener(ctx)
	})
}

func (s *podmanService) getCachedContainers() ([]podmanContainer, string) {
	s.mu.RLock()
	if s.initialized {
		containers := make([]podmanContainer, len(s.containers))
		copy(containers, s.containers)
		errMessage := s.errMessage
		s.mu.RUnlock()
		return containers, errMessage
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		containers := make([]podmanContainer, len(s.containers))
		copy(containers, s.containers)
		return containers, s.errMessage
	}

	containers, err := listPodmanContainers()
	if err != nil {
		message := podmanLoadFailedMessage
		if errors.Is(err, errPodmanUnavailable) {
			message = podmanUnavailableMessage
		}
		s.errMessage = message
		s.initialized = true
		return nil, message
	}

	normalizeContainers(containers)
	s.hash = hashContainers(containers)
	stored := make([]podmanContainer, len(containers))
	copy(stored, containers)
	s.containers = stored
	s.errMessage = ""
	s.initialized = true

	return containers, ""
}

func (s *podmanService) addClient(conn *websocket.Conn) *podmanClient {
	c := &podmanClient{
		conn:    conn,
		sendCh:  make(chan podmanStreamMessage, podmanClientBufferSize),
		closeCh: make(chan struct{}),
	}
	go c.writePump()

	s.hubMu.Lock()
	s.clients[c] = struct{}{}
	s.hubMu.Unlock()

	return c
}

func (s *podmanService) removeClient(c *podmanClient) {
	s.hubMu.Lock()
	delete(s.clients, c)
	s.hubMu.Unlock()

	c.close()
}

func (s *podmanService) broadcast(msg podmanStreamMessage) {
	s.hubMu.Lock()
	clients := make([]*podmanClient, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.hubMu.Unlock()

	for _, c := range clients {
		c.trySend(msg)
	}
}

func (c *podmanClient) trySend(msg podmanStreamMessage) {
	select {
	case c.sendCh <- msg:
	default:
		c.close()
	}
}

func (c *podmanClient) close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
}

func (c *podmanClient) writePump() {
	defer c.conn.Close()
	for {
		select {
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			if err := c.conn.SetWriteDeadline(time.Now().Add(podmanWriteTimeout)); err != nil {
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-c.closeCh:
			return
		}
	}
}

var podmanStreamUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}

		originURL, err := url.Parse(origin)
		if err != nil || originURL.Host == "" {
			return false
		}

		originHost := stripHostPort(originURL.Host)
		requestHost := stripHostPort(r.Host)
		if originHost == "" || requestHost == "" {
			return false
		}

		if strings.EqualFold(originHost, requestHost) {
			return true
		}

		return isLoopbackHost(originHost) && isLoopbackHost(requestHost)
	},
}

func registerPodmanRoutes(rtr *router.Router[*core.RequestEvent], svc *podmanService) {
	rtr.GET("/podman/containers", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return re.JSON(http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized.",
			})
		}

		containers, errMessage := svc.getCachedContainers()
		if errMessage != "" {
			status := http.StatusInternalServerError
			if errMessage == podmanUnavailableMessage {
				status = http.StatusServiceUnavailable
			}
			return re.JSON(status, map[string]string{
				"message": errMessage,
			})
		}

		return re.JSON(http.StatusOK, containers)
	})

	rtr.GET("/podman/containers/stream", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return re.JSON(http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized.",
			})
		}

		conn, err := podmanStreamUpgrader.Upgrade(re.Response, re.Request, nil)
		if err != nil {
			return err
		}

		client := svc.addClient(conn)
		defer svc.removeClient(client)

		containers, errMessage := svc.getCachedContainers()
		if errMessage != "" {
			client.trySend(podmanStreamMessage{Type: "error", Data: []podmanContainer{}, Message: errMessage})
		} else {
			client.trySend(podmanStreamMessage{Type: "containers", Data: containers})
		}

		svc.schedulePoll(podmanPollDebounce)

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return nil
			}
		}
	})

	registerWorkspaceRoutes(rtr, svc)
}

func stripHostPort(host string) string {
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
