package join

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
)

func TestJoinIssuesNodeCertificateAndConsumesToken(t *testing.T) {
	tempDir := t.TempDir()
	caCertPath, caKeyPath := writeTestCA(t, tempDir)

	database, err := db.Init(filepath.Join(tempDir, "panel.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer database.Close()

	service := NewService(database, caCertPath, caKeyPath, "panel.example.com:50051")
	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"admin-user",
		"admin",
		"hash",
		"admin",
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	token, err := service.CreateToken("admin-user", "game-node-01")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	csrPEM := createCSR(t)
	result, err := service.Join(JoinRequest{
		Token:  token.Token,
		CSRPem: csrPEM,
	})
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	if result.NodeID == "" {
		t.Fatalf("expected node id")
	}
	if result.PanelAddr != "panel.example.com:50051" {
		t.Fatalf("unexpected panel addr: %s", result.PanelAddr)
	}
	if !strings.Contains(result.CACertPEM, "BEGIN CERTIFICATE") {
		t.Fatalf("expected ca cert PEM")
	}

	cert := parseCertificatePEM(t, result.CertPEM)
	if cert.Subject.CommonName != result.NodeID {
		t.Fatalf("cert CN = %q, want %q", cert.Subject.CommonName, result.NodeID)
	}
	if len(cert.URIs) != 1 || cert.URIs[0].String() != "deft:node:"+result.NodeID {
		t.Fatalf("cert missing node URI: %v", cert.URIs)
	}

	var storedName string
	var storedFingerprint string
	err = database.QueryRow(
		"SELECT name, cert_fingerprint FROM nodes WHERE id = ?",
		result.NodeID,
	).Scan(&storedName, &storedFingerprint)
	if err != nil {
		t.Fatalf("load node: %v", err)
	}
	if storedName != "game-node-01" {
		t.Fatalf("stored name = %q", storedName)
	}
	if storedFingerprint == "" {
		t.Fatalf("expected cert fingerprint")
	}

	if _, err := service.Join(JoinRequest{Token: token.Token, CSRPem: csrPEM}); err == nil {
		t.Fatalf("expected second join with same token to fail")
	}
}

func TestListAndRevokeJoinTokens(t *testing.T) {
	tempDir := t.TempDir()
	caCertPath, caKeyPath := writeTestCA(t, tempDir)

	database, err := db.Init(filepath.Join(tempDir, "panel.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"admin-user",
		"admin",
		"hash",
		"admin",
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	service := NewService(database, caCertPath, caKeyPath, "panel.example.com:50051")
	token, err := service.CreateToken("admin-user", "temporary-node")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	tokens, err := service.ListTokens()
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("token count = %d, want 1", len(tokens))
	}
	if tokens[0].Status != "active" {
		t.Fatalf("token status = %q", tokens[0].Status)
	}

	if err := service.RevokeToken(tokens[0].ID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	tokens, err = service.ListTokens()
	if err != nil {
		t.Fatalf("list revoked tokens: %v", err)
	}
	if tokens[0].Status != "revoked" {
		t.Fatalf("token status = %q", tokens[0].Status)
	}

	if _, err := service.Join(JoinRequest{Token: token.Token, CSRPem: createCSR(t)}); err == nil {
		t.Fatalf("expected revoked token join to fail")
	}
}

func TestListTokensLimitsVisibleRowsAndHidesOldRevoked(t *testing.T) {
	tempDir := t.TempDir()
	caCertPath, caKeyPath := writeTestCA(t, tempDir)

	database, err := db.Init(filepath.Join(tempDir, "panel.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"admin-user",
		"admin",
		"hash",
		"admin",
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	now := time.Now()
	for i := 0; i < 2; i++ {
		if _, err := database.Exec(
			`INSERT INTO join_tokens (id, token_hash, node_name, created_by, created_at, expires_at, revoked_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"old-revoked-token-"+string(rune('a'+i)),
			"old-revoked-token-hash-"+string(rune('a'+i)),
			"old-revoked",
			"admin-user",
			now.Add(-time.Hour).Unix(),
			now.Add(time.Hour).Unix(),
			now.Add(-RevokedTokenVisibleDuration-time.Second).Unix(),
		); err != nil {
			t.Fatalf("insert old revoked token: %v", err)
		}
	}

	service := NewService(database, caCertPath, caKeyPath, "panel.example.com:50051")
	for i := 0; i < VisibleTokenLimit; i++ {
		if _, err := service.CreateToken("admin-user", "visible-node"); err != nil {
			t.Fatalf("create visible token %d: %v", i, err)
		}
	}

	tokens, err := service.ListTokens()
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(tokens) != VisibleTokenLimit {
		t.Fatalf("token count = %d, want %d", len(tokens), VisibleTokenLimit)
	}
	for _, token := range tokens {
		if token.Status != "active" {
			t.Fatalf("unexpected visible token status: %q", token.Status)
		}
	}
}

func TestJoinRequestApprovalReturnsCertificateToPollingAgent(t *testing.T) {
	tempDir := t.TempDir()
	caCertPath, caKeyPath := writeTestCA(t, tempDir)

	database, err := db.Init(filepath.Join(tempDir, "panel.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)",
		"admin-user",
		"admin",
		"hash",
		"admin",
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	service := NewService(database, caCertPath, caKeyPath, "panel.example.com:50051")
	request, err := service.CreateRequest("browser-node", createCSR(t), "https://panel.example.com")
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if request.ApprovalURL != "https://panel.example.com/nodes/join/"+request.ID {
		t.Fatalf("approval url = %q", request.ApprovalURL)
	}

	status, err := service.RequestStatus(request.ID, request.Secret)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Status != "pending" {
		t.Fatalf("status = %q", status.Status)
	}

	approved, err := service.ApproveRequest(request.ID, "admin-user")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.NodeID == "" {
		t.Fatalf("expected approved node id")
	}

	status, err = service.RequestStatus(request.ID, request.Secret)
	if err != nil {
		t.Fatalf("approved status: %v", err)
	}
	if status.Status != "approved" || status.Result == nil {
		t.Fatalf("unexpected approved status: %+v", status)
	}
	if status.Result.NodeID != approved.NodeID {
		t.Fatalf("status node id = %q, want %q", status.Result.NodeID, approved.NodeID)
	}

	if _, err := service.ApproveRequest(request.ID, "admin-user"); err == nil {
		t.Fatalf("expected second approval to fail")
	}
}

func writeTestCA(t *testing.T, dir string) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Deft Test CA"},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create ca cert: %v", err)
	}

	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	writePEM(t, certPath, "CERTIFICATE", der)
	writePEM(t, keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))

	return certPath, keyPath
}

func createCSR(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate agent key: %v", err)
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "joining-node"},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		t.Fatalf("create csr: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

func parseCertificatePEM(t *testing.T, certPEM string) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatalf("parse cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return cert
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()

	data := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
