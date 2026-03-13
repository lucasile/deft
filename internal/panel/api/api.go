package api

import (
	"net/http"

	"github.com/lucasile/deft/internal/panel/nodes"
	"github.com/lucasile/deft/internal/proto"
)

type Server struct {
	nodeManager *nodes.Manager
}

func NewServer(nodeManager *nodes.Manager) *Server {
	return &Server{
		nodeManager: nodeManager,
	}
}

func (s *Server) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/test-command", s.handleTestCommand)
}

func (s *Server) handleTestCommand(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node")
	if nodeID == "" {
		http.Error(w, "missing node parameter", http.StatusBadRequest)
		return
	}

	cmd := &proto.PanelCommand{
		CommandId: "test-cmd-" + nodeID,
		Action: &proto.PanelCommand_Start{
			Start: &proto.StartContainer{Id: "test-container"},
		},
	}

	if err := s.nodeManager.SendCommand(nodeID, cmd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Command sent"))
}
