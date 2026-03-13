package main

import (
	"embed"
	"io/fs"
	"net"
	"net/http"
	"os"

	"github.com/lucasile/deft/panel/internal/api"
	"github.com/lucasile/deft/panel/internal/db"
	"github.com/lucasile/deft/panel/internal/nodes"
	"github.com/lucasile/deft/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

//go:embed web/build/*
var webAssets embed.FS

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	database, err := db.Init("panel.db")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer database.Close()

	nodeManager := nodes.NewManager(database)
	apiServer := api.NewServer(nodeManager)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer()
	proto.RegisterAgentServiceServer(grpcServer, nodeManager)

	go func() {
		log.Info().Msg("gRPC server listening on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	mux := http.NewServeMux()
	apiServer.RegisterHandlers(mux)

	isDev := os.Getenv("DEV") == "true"

	if !isDev {
		// Setup HTTP server for production (embedded assets)
		publicFS, err := fs.Sub(webAssets, "web/build")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get subdirectory for web assets")
		}
		fileServer := http.FileServer(http.FS(publicFS))
		mux.Handle("/", fileServer)
	} else {
		log.Info().Msg("Running in DEV mode - Skipping embedded assets")
	}

	// Simple CORS wrapper for dev
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isDev {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				return
			}
		}
		mux.ServeHTTP(w, r)
	})

	addr := ":3000"
	log.Info().Str("addr", addr).Msg("Deft Panel API starting...")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal().Err(err).Msg("HTTP server failed")
	}
}
