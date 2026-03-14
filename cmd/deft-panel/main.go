package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/lucasile/deft/internal/panel/api"
	"github.com/lucasile/deft/internal/panel/db"
	"github.com/lucasile/deft/internal/panel/nodes"
	"github.com/lucasile/deft/internal/panel/ui"
	"github.com/lucasile/deft/internal/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	isDev := os.Getenv("DEV") == "true"
	
	httpPort := getEnv("DEFT_HTTP_PORT", "3000")
	grpcPort := getEnv("DEFT_GRPC_PORT", "50051")
	dbPath := getEnv("DEFT_DB_PATH", "panel.db")

	caPath := getEnv("DEFT_CA_PATH", "/etc/deft/certs/ca.crt")
	certPath := getEnv("DEFT_CERT_PATH", "/etc/deft/certs/panel.crt")
	keyPath := getEnv("DEFT_KEY_PATH", "/etc/deft/certs/panel.key")

	database, err := db.Init(dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer database.Close()

	nodeManager := nodes.NewManager(database)
	apiServer := api.NewServer(nodeManager)

	// Start gRPC server with mTLS
	grpcAddr := ":" + grpcPort
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", grpcAddr).Msg("failed to listen for gRPC")
	}

	creds, err := loadServerTLSCredentials(caPath, certPath, keyPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load TLS credentials, gRPC will be insecure")
	}

	var opts []grpc.ServerOption
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(opts...)
	proto.RegisterAgentServiceServer(grpcServer, nodeManager)

	go func() {
		log.Info().Str("addr", grpcAddr).Msg("gRPC server listening")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	mux := http.NewServeMux()
	apiServer.RegisterHandlers(mux)

	if !isDev {
		publicFS, err := ui.GetFS()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get ui assets")
		}
		mux.Handle("/", http.FileServer(http.FS(publicFS)))
	} else {
		log.Info().Msg("Running in DEV mode - skipping embedded assets")
	}

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

	httpAddr := ":" + httpPort
	log.Info().Str("addr", httpAddr).Msg("Deft Panel starting...")

	if err := http.ListenAndServe(httpAddr, handler); err != nil {
		log.Fatal().Err(err).Msg("http server failed")
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func loadServerTLSCredentials(caPath, certPath, keyPath string) (credentials.TransportCredentials, error) {
	// Load CA certificate
	pemClientCA, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, fmt.Errorf("failed to add client CA's certificate")
	}

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(config), nil
}
