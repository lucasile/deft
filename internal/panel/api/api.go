package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lucasile/deft/internal/panel/audit"
	"github.com/lucasile/deft/internal/panel/auth"
	"github.com/lucasile/deft/internal/panel/nodes"
	"github.com/lucasile/deft/internal/proto"
	"github.com/rs/zerolog/log"
)

type Server struct {
	nodeManager   *nodes.Manager
	auth          *auth.Service
	audit         *audit.Logger
	secureCookies bool
	authLimiter   *rateLimiter
	actionLimiter *rateLimiter
}

func NewServer(nodeManager *nodes.Manager, authService *auth.Service, auditLogger *audit.Logger, secureCookies bool) *Server {
	return &Server{
		nodeManager:   nodeManager,
		auth:          authService,
		audit:         auditLogger,
		secureCookies: secureCookies,
		authLimiter:   newRateLimiter(5, time.Minute),
		actionLimiter: newRateLimiter(60, time.Minute),
	}
}

func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", s.rateLimitAuth(s.handleRegister))
	mux.HandleFunc("POST /api/auth/login", s.rateLimitAuth(s.handleLogin))
	mux.HandleFunc("GET /api/auth/csrf", s.requireAuth(s.handleCSRF))
	mux.HandleFunc("POST /api/auth/logout", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleLogout))))
	mux.HandleFunc("GET /api/nodes", s.requireAuth(s.handleListNodes))
	mux.HandleFunc("GET /api/commands/{commandID}", s.requireAuth(s.handleGetCommand))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleCreateContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/start", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleStartContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/stop", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleStopContainer))))
	mux.HandleFunc("POST /api/nodes/{nodeID}/containers/{containerID}/remove", s.rateLimitAction(s.requireAuth(s.requireCSRF(s.handleRemoveContainer))))
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

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodeList, err := s.nodeManager.ListNodes()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, nodeList)
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

	cmd := &proto.PanelCommand{
		CommandId: commandID,
		Action: &proto.PanelCommand_Create{
			Create: &proto.CreateContainer{
				Name:  req.Name,
				Image: req.Image,
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

	if err := s.nodeManager.UpsertContainer(nodeID, req.Name, req.Name, req.Image, "create_requested"); err != nil {
		s.auditCurrentUser(r, "container.create", nodeID, req.Name, commandID, false, err.Error())
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

	if err := s.nodeManager.UpdateContainerStatus(nodeID, containerID, status); err != nil {
		s.auditCurrentUser(r, action, nodeID, containerID, commandID, false, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
