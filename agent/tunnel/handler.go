package tunnel

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/lucasile/deft/agent/docker"
	dockercontainer "github.com/lucasile/deft/agent/docker/container"
	"github.com/lucasile/deft/proto"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	docker *docker.Client
	conn   *Connection
	nodeID string
}

func NewHandler(docker *docker.Client, conn *Connection, nodeID string) *Handler {
	return &Handler{
		docker: docker,
		conn:   conn,
		nodeID: nodeID,
	}
}

func (h *Handler) HandleCommand(ctx context.Context, cmd *proto.PanelCommand) error {
	var err error
	var msg string

	switch a := cmd.Action.(type) {
	case *proto.PanelCommand_Create:
		_, err = dockercontainer.Create(ctx, h.docker, a.Create.Name, a.Create.Image, &container.Config{
			Image: a.Create.Image,
		}, &container.HostConfig{})
		msg = "Container created"
	case *proto.PanelCommand_Start:
		err = dockercontainer.Start(ctx, h.docker, a.Start.Id)
		msg = "Container started"
	case *proto.PanelCommand_Stop:
		err = dockercontainer.Stop(ctx, h.docker, a.Stop.Id)
		msg = "Container stopped"
	case *proto.PanelCommand_Remove:
		err = dockercontainer.Remove(ctx, h.docker, a.Remove.Id)
		msg = "Container removed"
	default:
		err = fmt.Errorf("unknown command type")
	}

	result := &proto.CommandResult{
		CommandId: cmd.CommandId,
		Success:   err == nil,
		Message:   msg,
	}

	if err != nil {
		result.Message = err.Error()
		log.Error().Err(err).Str("command_id", cmd.CommandId).Msg("Command failed")
	}

	return h.conn.stream.Send(&proto.AgentMessage{
		NodeId: h.nodeID,
		Content: &proto.AgentMessage_Result{
			Result: result,
		},
	})
}
