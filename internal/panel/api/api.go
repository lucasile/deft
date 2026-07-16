package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lucasile/deft/internal/panel/audit"
	"github.com/lucasile/deft/internal/panel/auth"
	"github.com/lucasile/deft/internal/panel/events"
	"github.com/lucasile/deft/internal/panel/join"
	"github.com/lucasile/deft/internal/panel/nodes"
	"github.com/lucasile/deft/internal/proto"
	"github.com/rs/zerolog/log"
)

type Server struct {
	nodeManager   *nodes.Manager
	auth          *auth.Service
	audit         *audit.Logger
	events        *events.Hub
	join          *join.Service
	secureCookies bool
	authLimiter   *rateLimiter
	actionLimiter *rateLimiter
	liveLogMu     sync.Mutex
	liveLogTokens map[string]liveLogToken
}

type liveLogToken struct {
	NodeID      string
	ContainerID string
	ExpiresAt   time.Time
}

func NewServer(
	nodeManager *nodes.Manager,
	authService *auth.Service,
	auditLogger *audit.Logger,
	eventHub *events.Hub,
	joinService *join.Service,
	secureCookies bool,
) *Server {
	return &Server{
		nodeManager:   nodeManager,
		auth:          authService,
		audit:         auditLogger,
		events:        eventHub,
		join:          joinService,
		secureCookies: secureCookies,
		authLimiter:   newRateLimiter(5, time.Minute),
		actionLimiter: newRateLimiter(60, time.Minute),
		liveLogTokens: make(map[string]liveLogToken),
	}
}

func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", s.rateLimitAuth(s.handleRegister))
	mux.HandleFunc("POST /api/auth/login", s.rateLimitAuth(s.handleLogin))
	mux.HandleFunc("GET /api/auth/csrf", s.requireAuth(s.handleCSRF))
	mux.HandleFunc("POST /api/auth/logout", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleLogout))))
	mux.HandleFunc("GET /api/agent/join-tokens", s.requireAuth(s.handleListJoinTokens))
	mux.HandleFunc("POST /api/agent/join-tokens", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleCreateJoinToken))))
	mux.HandleFunc("DELETE /api/agent/join-tokens/{tokenID}", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleRevokeJoinToken))))
	mux.HandleFunc("POST /api/agent/join", s.rateLimitAuth(s.handleAgentJoin))
	mux.HandleFunc("POST /api/agent/join-requests", s.rateLimitAuth(s.handleCreateJoinRequest))
	mux.HandleFunc("GET /api/agent/join-requests/{requestID}", s.rateLimitAction(s.handleJoinRequestStatus))
	mux.HandleFunc("GET /api/agent/join-requests/{requestID}/review", s.requireAuth(s.handleReviewJoinRequest))
	mux.HandleFunc("POST /api/agent/join-requests/{requestID}/approve", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleApproveJoinRequest))))
	mux.HandleFunc("GET /api/nodes", s.requireAuth(s.handleListNodes))
	mux.HandleFunc("DELETE /api/nodes/{nodeID}", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleRemoveNode))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/stop", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleStopNode))))
	mux.HandleFunc("GET /api/events", s.requireAuth(s.handleEvents))
	mux.HandleFunc("GET /api/commands", s.requireAuth(s.handleListCommands))
	mux.HandleFunc("GET /api/commands/{commandID}", s.requireAuth(s.handleGetCommand))
	mux.HandleFunc("GET /api/nodes/{nodeID}/containers", s.requireAuth(s.handleListContainers))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleCreateContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/start", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleStartContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/stop", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleStopContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/remove", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleRemoveContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/logs", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleContainerLogs))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/logs/stream", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleCreateContainerLogStream))))
	mux.HandleFunc("GET /api/nodes/{nodeID}/containers/{containerID}/logs/stream", s.rateLimitAction(s.requireAuth(s.handleContainerLogStream)))
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	User      *auth.User `json:"user"`
	CSRFToken string     `json:"csrf_token,omitempty"`
}

type csrfResponse struct {
	CSRFToken string `json:"csrf_token"`
}

type createContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type commandResponse struct {
	CommandID string `json:"command_id"`
}

type logStreamResponse struct {
	StreamID string `json:"stream_id"`
}

type createJoinTokenRequest struct {
	NodeName string `json:"node_name"`
}

type agentJoinRequest struct {
	NodeName string `json:"node_name"`
	CSRPem   string `json:"csr_pem"`
}

type createJoinRequestRequest struct {
	NodeName string `json:"node_name"`
	CSRPem   string `json:"csr_pem"`
	PanelURL string `json:"panel_url"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	user, err := s.auth.RegisterFirstUser(req.Username, req.Password)
	if err != nil {
		s.recordAudit(r, audit.Event{
			Action:  "auth.register",
			Success: false,
			Message: err.Error(),
		})
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	s.recordAudit(r, audit.Event{
		User:    user,
		Action:  "auth.register",
		Success: true,
	})
	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	session, user, err := s.auth.Login(req.Username, req.Password)
	if err != nil {
		s.recordAudit(r, audit.Event{
			Action:   "auth.login",
			TargetID: strings.TrimSpace(req.Username),
			Success:  false,
			Message:  "invalid username or password",
		})
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	s.recordAudit(r, audit.Event{
		User:    user,
		Action:  "auth.login",
		Success: true,
	})
	http.SetCookie(w, sessionCookie(session, s.secureCookies))
	writeJSON(w, http.StatusOK, authResponse{User: user, CSRFToken: session.CSRFToken})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookie)
	if err == nil {
		if err := s.auth.DeleteSession(cookie.Value); err != nil {
			s.auditCurrentUser(r, "auth.logout", "", "", "", false, err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	s.auditCurrentUser(r, "auth.logout", "", "", "", true, "")
	http.SetCookie(w, expiredSessionCookie(s.secureCookies))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCSRF(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookie)
	if err != nil {
		http.Error(w, "missing session", http.StatusUnauthorized)
		return
	}

	csrfToken, err := s.auth.CSRFToken(cookie.Value)
	if err != nil {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, csrfResponse{CSRFToken: csrfToken})
}

func (s *Server) handleCreateJoinToken(w http.ResponseWriter, r *http.Request) {
	var req createJoinTokenRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.auditCurrentUser(r, "agent.join_token.create", "", "", "", false, "invalid json body")
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.NodeName != "" {
		if err := validateNodeName(req.NodeName); err != nil {
			s.auditCurrentUser(r, "agent.join_token.create", "", req.NodeName, "", false, err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	user, _ := auth.UserFromContext(r.Context())
	if err := s.join.CheckCA(); err != nil {
		s.auditCurrentUser(r, "agent.join_token.create", "", req.NodeName, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	token, err := s.join.CreateToken(user.ID, req.NodeName)
	if err != nil {
		s.auditCurrentUser(r, "agent.join_token.create", "", req.NodeName, "", false, err.Error())
		status := http.StatusInternalServerError
		if errors.Is(err, join.ErrActiveTokenLimit) {
			status = http.StatusTooManyRequests
		}
		http.Error(w, err.Error(), status)
		return
	}

	s.auditCurrentUser(r, "agent.join_token.create", "", req.NodeName, "", true, "")
	writeJSON(w, http.StatusCreated, token)
}

func (s *Server) handleListJoinTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.join.ListTokens()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleRevokeJoinToken(w http.ResponseWriter, r *http.Request) {
	tokenID := strings.TrimSpace(r.PathValue("tokenID"))
	if err := validateJoinTokenID(tokenID); err != nil {
		s.auditCurrentUser(r, "agent.join_token.revoke", "", tokenID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.join.RevokeToken(tokenID); err != nil {
		s.auditCurrentUser(r, "agent.join_token.revoke", "", tokenID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	s.auditCurrentUser(r, "agent.join_token.revoke", "", tokenID, "", true, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAgentJoin(w http.ResponseWriter, r *http.Request) {
	var req agentJoinRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.recordAudit(r, audit.Event{
			Action:  "agent.join",
			Success: false,
			Message: "invalid json body",
		})
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.NodeName != "" {
		if err := validateNodeName(req.NodeName); err != nil {
			s.recordAudit(r, audit.Event{
				Action:   "agent.join",
				TargetID: req.NodeName,
				Success:  false,
				Message:  err.Error(),
			})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	result, err := s.join.Join(join.JoinRequest{
		Token:    bearerToken(r),
		NodeName: req.NodeName,
		CSRPem:   req.CSRPem,
	})
	if err != nil {
		s.recordAudit(r, audit.Event{
			Action:   "agent.join",
			TargetID: req.NodeName,
			Success:  false,
			Message:  err.Error(),
		})
		status := http.StatusForbidden
		if join.IsCAUnavailable(err) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}

	s.recordAudit(r, audit.Event{
		Action:   "agent.join",
		NodeID:   result.NodeID,
		TargetID: req.NodeName,
		Success:  true,
	})
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleCreateJoinRequest(w http.ResponseWriter, r *http.Request) {
	var req createJoinRequestRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.recordAudit(r, audit.Event{
			Action:  "agent.join_request.create",
			Success: false,
			Message: "invalid json body",
		})
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.NodeName != "" {
		if err := validateNodeName(req.NodeName); err != nil {
			s.recordAudit(r, audit.Event{
				Action:   "agent.join_request.create",
				TargetID: req.NodeName,
				Success:  false,
				Message:  err.Error(),
			})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	panelURL := strings.TrimSpace(req.PanelURL)
	if panelURL == "" {
		panelURL = "https://" + r.Host
	}
	if err := s.join.CheckCA(); err != nil {
		s.recordAudit(r, audit.Event{
			Action:   "agent.join_request.create",
			TargetID: req.NodeName,
			Success:  false,
			Message:  err.Error(),
		})
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	result, err := s.join.CreateRequest(req.NodeName, req.CSRPem, panelURL)
	if err != nil {
		s.recordAudit(r, audit.Event{
			Action:   "agent.join_request.create",
			TargetID: req.NodeName,
			Success:  false,
			Message:  err.Error(),
		})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.recordAudit(r, audit.Event{
		Action:   "agent.join_request.create",
		TargetID: req.NodeName,
		Success:  true,
	})
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleJoinRequestStatus(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if err := validateJoinRequestID(requestID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status, err := s.join.RequestStatus(requestID, bearerToken(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleApproveJoinRequest(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if err := validateJoinRequestID(requestID); err != nil {
		s.auditCurrentUser(r, "agent.join_request.approve", "", requestID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, _ := auth.UserFromContext(r.Context())
	result, err := s.join.ApproveRequest(requestID, user.ID)
	if err != nil {
		s.auditCurrentUser(r, "agent.join_request.approve", "", requestID, "", false, err.Error())
		status := http.StatusForbidden
		if join.IsCAUnavailable(err) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), status)
		return
	}

	s.auditCurrentUser(r, "agent.join_request.approve", result.NodeID, requestID, "", true, "")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReviewJoinRequest(w http.ResponseWriter, r *http.Request) {
	requestID := strings.TrimSpace(r.PathValue("requestID"))
	if err := validateJoinRequestID(requestID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	review, err := s.join.ReviewRequest(requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, review)
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodeList, err := s.nodeManager.ListNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, nodeList)
}

func (s *Server) handleRemoveNode(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	if err := validateNodeID(nodeID); err != nil {
		s.auditCurrentUser(r, "node.remove", nodeID, "", "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.nodeManager.RemoveNode(nodeID); err != nil {
		s.auditCurrentUser(r, "node.remove", nodeID, "", "", false, err.Error())
		status := http.StatusForbidden
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	s.auditCurrentUser(r, "node.remove", nodeID, "", "", true, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStopNode(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	if err := validateNodeID(nodeID); err != nil {
		s.auditCurrentUser(r, "agent.stop", nodeID, "", "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commandID, err := newCommandID()
	if err != nil {
		s.auditCurrentUser(r, "agent.stop", nodeID, "", "", false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.nodeManager.CreateCommand(commandID, nodeID, "agent.stop", nodeID); err != nil {
		s.auditCurrentUser(r, "agent.stop", nodeID, nodeID, commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cmd := &proto.PanelCommand{
		CommandId: commandID,
		Action: &proto.PanelCommand_Shutdown{
			Shutdown: &proto.ShutdownAgent{},
		},
	}
	if err := s.nodeManager.SendCommand(nodeID, cmd); err != nil {
		s.auditCurrentUser(r, "agent.stop", nodeID, nodeID, commandID, false, err.Error())
		_ = s.nodeManager.CompleteCommand(commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.auditCurrentUser(r, "agent.stop", nodeID, nodeID, commandID, true, "")
	writeJSON(w, http.StatusAccepted, commandResponse{CommandID: commandID})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	eventCh, unsubscribe := s.events.Subscribe()
	defer unsubscribe()

	if err := events.WriteSSE(w, events.Event{Name: "ready"}); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if err := events.WriteSSE(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleListCommands(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		value, err := strconv.Atoi(rawLimit)
		if err != nil || value <= 0 || value > 200 {
			http.Error(w, "limit must be between 1 and 200", http.StatusBadRequest)
			return
		}
		limit = value
	}

	commands, err := s.nodeManager.ListCommands(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, commands)
}

func (s *Server) handleGetCommand(w http.ResponseWriter, r *http.Request) {
	commandID := strings.TrimSpace(r.PathValue("commandID"))
	if err := validateCommandID(commandID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	command, err := s.nodeManager.GetCommand(commandID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, command)
}

func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	s.handleContainerCommand(w, r, func(commandID, containerID string) *proto.PanelCommand {
		return &proto.PanelCommand{
			CommandId: commandID,
			Action: &proto.PanelCommand_Logs{
				Logs: &proto.GetContainerLogs{
					Id:        containerID,
					TailLines: 200,
				},
			},
		}
	}, "container.logs", "")
}

func (s *Server) handleCreateContainerLogStream(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	containerID := strings.TrimSpace(r.PathValue("containerID"))
	if err := validateNodeID(nodeID); err != nil {
		s.auditCurrentUser(r, "container.logs.stream.create", nodeID, containerID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateContainerID(containerID); err != nil {
		s.auditCurrentUser(r, "container.logs.stream.create", nodeID, containerID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	streamID, err := newCommandID()
	if err != nil {
		s.auditCurrentUser(r, "container.logs.stream.create", nodeID, containerID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	s.liveLogMu.Lock()
	s.pruneExpiredLiveLogTokens(now)
	s.liveLogTokens[streamID] = liveLogToken{
		NodeID:      nodeID,
		ContainerID: containerID,
		ExpiresAt:   now.Add(2 * time.Minute),
	}
	s.liveLogMu.Unlock()

	s.auditCurrentUser(r, "container.logs.stream.create", nodeID, containerID, streamID, true, "")
	writeJSON(w, http.StatusCreated, logStreamResponse{StreamID: streamID})
}

func (s *Server) handleContainerLogStream(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	containerID := strings.TrimSpace(r.PathValue("containerID"))
	if err := validateNodeID(nodeID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateContainerID(containerID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	streamID := strings.TrimSpace(r.URL.Query().Get("stream_id"))
	if err := validateCommandID(streamID); err != nil {
		http.Error(w, "invalid stream id", http.StatusBadRequest)
		return
	}
	if !s.consumeLiveLogToken(streamID, nodeID, containerID) {
		http.Error(w, "invalid or expired log stream", http.StatusForbidden)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	eventCh, unsubscribe := s.events.Subscribe()
	defer unsubscribe()

	cmd := &proto.PanelCommand{
		CommandId: streamID,
		Action: &proto.PanelCommand_FollowLogs{
			FollowLogs: &proto.FollowContainerLogs{
				Id:        containerID,
				TailLines: 200,
				StreamId:  streamID,
			},
		},
	}
	if err := s.nodeManager.SendCommand(nodeID, cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		_ = s.nodeManager.SendCommand(nodeID, &proto.PanelCommand{
			Action: &proto.PanelCommand_CancelLogs{
				CancelLogs: &proto.CancelLogStream{StreamId: streamID},
			},
		})
	}()

	s.auditCurrentUser(r, "container.logs.follow", nodeID, containerID, streamID, true, "")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := events.WriteSSE(w, events.Event{Name: "ready", Data: map[string]string{"stream_id": streamID}}); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if event.Name != nodes.EventLogChunk {
				continue
			}
			chunk, ok := event.Data.(nodes.LogChunk)
			if !ok || chunk.StreamID != streamID || chunk.NodeID != nodeID || chunk.ContainerID != containerID {
				continue
			}
			if err := events.WriteSSE(w, events.Event{Name: nodes.EventLogChunk, Data: chunk}); err != nil {
				return
			}
			flusher.Flush()
			if chunk.EOF {
				return
			}
		}
	}
}

func (s *Server) consumeLiveLogToken(streamID, nodeID, containerID string) bool {
	now := time.Now()
	s.liveLogMu.Lock()
	defer s.liveLogMu.Unlock()

	s.pruneExpiredLiveLogTokens(now)
	token, ok := s.liveLogTokens[streamID]
	if !ok {
		return false
	}
	delete(s.liveLogTokens, streamID)
	return token.NodeID == nodeID && token.ContainerID == containerID && token.ExpiresAt.After(now)
}

func (s *Server) pruneExpiredLiveLogTokens(now time.Time) {
	for streamID, token := range s.liveLogTokens {
		if !token.ExpiresAt.After(now) {
			delete(s.liveLogTokens, streamID)
		}
	}
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	if err := validateNodeID(nodeID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	containers, err := s.nodeManager.ListContainers(nodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, containers)
}

func (s *Server) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	if err := validateNodeID(nodeID); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, "", "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req createContainerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, "", "", false, "invalid json body")
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	if err := validateContainerName(req.Name); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, req.Name, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateImage(req.Image); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, req.Name, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commandID, err := newCommandID()
	if err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, req.Name, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dockerName := dockerContainerName(nodeID, commandID)

	cmd := &proto.PanelCommand{
		CommandId: commandID,
		Action: &proto.PanelCommand_Create{
			Create: &proto.CreateContainer{
				Name:        dockerName,
				Image:       req.Image,
				DisplayName: req.Name,
				ResourceId:  commandID,
			},
		},
	}

	if err := s.nodeManager.CreateCommand(commandID, nodeID, "container.create", req.Name); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, req.Name, commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.nodeManager.SendCommand(nodeID, cmd); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, req.Name, commandID, false, err.Error())
		_ = s.nodeManager.CompleteCommand(commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.auditCurrentUser(r, "container.create", nodeID, req.Name, commandID, true, "")
	writeJSON(w, http.StatusAccepted, commandResponse{CommandID: commandID})
}

func (s *Server) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerCommand(w, r, func(commandID, containerID string) *proto.PanelCommand {
		return &proto.PanelCommand{
			CommandId: commandID,
			Action: &proto.PanelCommand_Start{
				Start: &proto.StartContainer{Id: containerID},
			},
		}
	}, "container.start", "start_requested")
}

func (s *Server) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerCommand(w, r, func(commandID, containerID string) *proto.PanelCommand {
		return &proto.PanelCommand{
			CommandId: commandID,
			Action: &proto.PanelCommand_Stop{
				Stop: &proto.StopContainer{Id: containerID},
			},
		}
	}, "container.stop", "stop_requested")
}

func (s *Server) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	s.handleContainerCommand(w, r, func(commandID, containerID string) *proto.PanelCommand {
		return &proto.PanelCommand{
			CommandId: commandID,
			Action: &proto.PanelCommand_Remove{
				Remove: &proto.RemoveContainer{Id: containerID},
			},
		}
	}, "container.remove", "remove_requested")
}

func (s *Server) handleContainerCommand(
	w http.ResponseWriter,
	r *http.Request,
	buildCommand func(commandID, containerID string) *proto.PanelCommand,
	action string,
	status string,
) {
	nodeID := strings.TrimSpace(r.PathValue("nodeID"))
	containerID := strings.TrimSpace(r.PathValue("containerID"))
	if err := validateNodeID(nodeID); err != nil {
		s.auditCurrentUser(r, action, nodeID, containerID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateContainerID(containerID); err != nil {
		s.auditCurrentUser(r, action, nodeID, containerID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commandID, err := newCommandID()
	if err != nil {
		s.auditCurrentUser(r, action, nodeID, containerID, "", false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.nodeManager.CreateCommand(commandID, nodeID, action, containerID); err != nil {
		s.auditCurrentUser(r, action, nodeID, containerID, commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.nodeManager.SendCommand(nodeID, buildCommand(commandID, containerID)); err != nil {
		s.auditCurrentUser(r, action, nodeID, containerID, commandID, false, err.Error())
		_ = s.nodeManager.CompleteCommand(commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if status != "" {
		if err := s.nodeManager.UpdateContainerStatus(nodeID, containerID, status); err != nil {
			s.auditCurrentUser(r, action, nodeID, containerID, commandID, false, err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	s.auditCurrentUser(r, action, nodeID, containerID, commandID, true, "")
	writeJSON(w, http.StatusAccepted, commandResponse{CommandID: commandID})
}

func (s *Server) rateLimitAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.rateLimit(s.authLimiter, "auth", next)
}

func (s *Server) rateLimitAction(next http.HandlerFunc) http.HandlerFunc {
	return s.rateLimit(s.actionLimiter, "action", next)
}

func (s *Server) rateLimit(limiter *rateLimiter, scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := scope + ":" + clientIP(r)
		if !limiter.Allow(key) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookie)
		if err != nil {
			http.Error(w, "missing session", http.StatusUnauthorized)
			return
		}

		user, err := s.auth.AuthenticateSession(cookie.Value)
		if err != nil {
			http.Error(w, "invalid session", http.StatusUnauthorized)
			return
		}
		if user.Role != auth.AdminRole {
			http.Error(w, "admin role required", http.StatusForbidden)
			return
		}

		next(w, r.WithContext(auth.WithUser(r.Context(), user)))
	}
}

func (s *Server) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookie)
		if err != nil {
			http.Error(w, "missing session", http.StatusUnauthorized)
			return
		}

		if err := s.auth.ValidateCSRFToken(cookie.Value, r.Header.Get("X-CSRF-Token")); err != nil {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func (s *Server) auditCurrentUser(r *http.Request, action, nodeID, targetID, commandID string, success bool, message string) {
	user, _ := auth.UserFromContext(r.Context())
	s.recordAudit(r, audit.Event{
		User:      user,
		Action:    action,
		NodeID:    nodeID,
		TargetID:  targetID,
		CommandID: commandID,
		Success:   success,
		Message:   message,
	})
}

func (s *Server) recordAudit(r *http.Request, event audit.Event) {
	event.RemoteAddr = clientIP(r)
	if err := s.audit.Record(event); err != nil {
		log.Error().Err(err).Str("action", event.Action).Msg("failed to write audit log")
	}
}

func sessionCookie(session *auth.Session, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func expiredSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func newCommandID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("failed to create command id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func dockerContainerName(nodeID, resourceID string) string {
	nodePart := strings.TrimPrefix(nodeID, "node_")
	if len(nodePart) > 8 {
		nodePart = nodePart[:8]
	}
	resourcePart := resourceID
	if len(resourcePart) > 12 {
		resourcePart = resourcePart[:12]
	}
	return "deft-" + nodePart + "-" + resourcePart
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return ""
	}
	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
