package join

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lucasile/deft/internal/panel/db"
)

const (
	TokenLifetime               = 10 * time.Minute
	ActiveTokenLimit            = 5
	VisibleTokenLimit           = 5
	RevokedTokenVisibleDuration = 5 * time.Minute
	nodeIDPrefix                = "node_"
)

var (
	ErrCAUnavailable    = errors.New("agent join CA unavailable")
	ErrActiveTokenLimit = errors.New("active join token limit reached")
)

type Service struct {
	db         *db.DB
	caCertPath string
	caKeyPath  string
	panelAddr  string
}

type Token struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type TokenInfo struct {
	ID           string `json:"id"`
	NodeName     string `json:"node_name,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    int64  `json:"expires_at"`
	UsedAt       *int64 `json:"used_at,omitempty"`
	RevokedAt    *int64 `json:"revoked_at,omitempty"`
	UsedByNodeID string `json:"used_by_node_id,omitempty"`
	Status       string `json:"status"`
}

type JoinRequest struct {
	Token    string
	NodeName string
	CSRPem   string
}

type JoinResult struct {
	NodeID    string `json:"node_id"`
	PanelAddr string `json:"panel_addr"`
	CACertPEM string `json:"ca_cert_pem"`
	CertPEM   string `json:"cert_pem"`
}

type Request struct {
	ID               string `json:"id"`
	Secret           string `json:"secret,omitempty"`
	VerificationCode string `json:"verification_code"`
	ApprovalURL      string `json:"approval_url"`
	ExpiresAt        int64  `json:"expires_at"`
}

type RequestStatus struct {
	Status string      `json:"status"`
	Result *JoinResult `json:"result,omitempty"`
}

type Review struct {
	ID               string `json:"id"`
	NodeName         string `json:"node_name,omitempty"`
	VerificationCode string `json:"verification_code"`
	ExpiresAt        int64  `json:"expires_at"`
	Status           string `json:"status"`
}

func NewService(database *db.DB, caCertPath, caKeyPath, panelAddr string) *Service {
	return &Service{
		db:         database,
		caCertPath: caCertPath,
		caKeyPath:  caKeyPath,
		panelAddr:  panelAddr,
	}
}

func (s *Service) CheckCA() error {
	if _, _, _, err := s.loadCA(); err != nil {
		return fmt.Errorf("%w: %w", ErrCAUnavailable, err)
	}
	return nil
}

func IsCAUnavailable(err error) bool {
	return errors.Is(err, ErrCAUnavailable)
}

func (s *Service) CreateToken(createdBy, nodeName string) (*Token, error) {
	activeTokens, err := s.ActiveTokenCount()
	if err != nil {
		return nil, err
	}
	if activeTokens >= ActiveTokenLimit {
		return nil, ErrActiveTokenLimit
	}

	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	id, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.Add(TokenLifetime)
	_, err = s.db.Exec(
		`INSERT INTO join_tokens (id, token_hash, node_name, created_by, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		hashToken(token),
		strings.TrimSpace(nodeName),
		createdBy,
		now.Unix(),
		expiresAt.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create join token: %w", err)
	}

	return &Token{Token: token, ExpiresAt: expiresAt.Unix()}, nil
}

func (s *Service) ListTokens() ([]TokenInfo, error) {
	now := time.Now().Unix()
	revokedAfter := time.Now().Add(-RevokedTokenVisibleDuration).Unix()
	rows, err := s.db.Query(
		`SELECT id, node_name, created_at, expires_at, used_at, revoked_at, used_by_node_id
		 FROM join_tokens
		 WHERE used_at IS NULL
		   AND ((revoked_at IS NULL AND expires_at > ?) OR revoked_at > ?)
		 ORDER BY created_at DESC
		 LIMIT ?`,
		now,
		revokedAfter,
		VisibleTokenLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list join tokens: %w", err)
	}
	defer rows.Close()

	var tokens []TokenInfo
	for rows.Next() {
		var token TokenInfo
		var nodeName sql.NullString
		var usedAt sql.NullInt64
		var revokedAt sql.NullInt64
		var usedByNodeID sql.NullString
		if err := rows.Scan(
			&token.ID,
			&nodeName,
			&token.CreatedAt,
			&token.ExpiresAt,
			&usedAt,
			&revokedAt,
			&usedByNodeID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan join token: %w", err)
		}
		if nodeName.Valid {
			token.NodeName = nodeName.String
		}
		if usedAt.Valid {
			value := usedAt.Int64
			token.UsedAt = &value
		}
		if revokedAt.Valid {
			value := revokedAt.Int64
			token.RevokedAt = &value
		}
		if usedByNodeID.Valid {
			token.UsedByNodeID = usedByNodeID.String
		}
		token.Status = tokenStatus(token.ExpiresAt, usedAt.Valid, revokedAt.Valid, now)
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read join tokens: %w", err)
	}

	return tokens, nil
}

func (s *Service) ActiveTokenCount() (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*)
		 FROM join_tokens
		 WHERE used_at IS NULL
		   AND revoked_at IS NULL
		   AND expires_at > ?`,
		time.Now().Unix(),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active join tokens: %w", err)
	}
	return count, nil
}

func (s *Service) RevokeToken(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("join token id is required")
	}

	result, err := s.db.Exec(
		`UPDATE join_tokens
		 SET revoked_at = ?
		 WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL`,
		time.Now().Unix(),
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to revoke join token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to confirm join token revoke: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("join token is already used, revoked, or not found")
	}

	return nil
}

func (s *Service) Join(req JoinRequest) (*JoinResult, error) {
	req.Token = strings.TrimSpace(req.Token)
	req.NodeName = strings.TrimSpace(req.NodeName)
	req.CSRPem = strings.TrimSpace(req.CSRPem)
	if req.Token == "" {
		return nil, fmt.Errorf("join token is required")
	}
	if req.CSRPem == "" {
		return nil, fmt.Errorf("csr_pem is required")
	}

	csr, err := parseCSR(req.CSRPem)
	if err != nil {
		return nil, err
	}

	nodeID, err := newNodeID()
	if err != nil {
		return nil, err
	}

	caCertPEM, caCert, caKey, err := s.loadCA()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCAUnavailable, err)
	}

	certPEM, fingerprint, subject, err := signAgentCSR(csr, caCert, caKey, nodeID)
	if err != nil {
		return nil, err
	}

	tokenID, defaultNodeName, err := s.consumeToken(req.Token)
	if err != nil {
		return nil, err
	}
	if req.NodeName == "" {
		req.NodeName = defaultNodeName
	}

	now := time.Now().Unix()
	_, err = s.db.Exec(
		`INSERT INTO nodes (id, name, last_seen, cert_fingerprint, cert_subject, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		nodeID,
		req.NodeName,
		now,
		fingerprint,
		subject,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	_, err = s.db.Exec(
		"UPDATE join_tokens SET used_by_node_id = ? WHERE id = ?",
		nodeID,
		tokenID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to finish join token use: %w", err)
	}

	return &JoinResult{
		NodeID:    nodeID,
		PanelAddr: s.panelAddr,
		CACertPEM: string(caCertPEM),
		CertPEM:   string(certPEM),
	}, nil
}

func (s *Service) CreateRequest(nodeName, csrPEM, panelURL string) (*Request, error) {
	nodeName = strings.TrimSpace(nodeName)
	csrPEM = strings.TrimSpace(csrPEM)
	if csrPEM == "" {
		return nil, fmt.Errorf("csr_pem is required")
	}
	if _, err := parseCSR(csrPEM); err != nil {
		return nil, err
	}

	id, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	secret, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	code, err := verificationCode()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.Add(TokenLifetime)
	_, err = s.db.Exec(
		`INSERT INTO join_requests (id, secret_hash, verification_code, node_name, csr_pem, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id,
		hashToken(secret),
		code,
		nodeName,
		csrPEM,
		now.Unix(),
		expiresAt.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create join request: %w", err)
	}

	return &Request{
		ID:               id,
		Secret:           secret,
		VerificationCode: code,
		ApprovalURL:      strings.TrimRight(panelURL, "/") + "/nodes/join/" + id,
		ExpiresAt:        expiresAt.Unix(),
	}, nil
}

func (s *Service) RequestStatus(id, secret string) (*RequestStatus, error) {
	id = strings.TrimSpace(id)
	secret = strings.TrimSpace(secret)
	if id == "" || secret == "" {
		return nil, fmt.Errorf("join request id and secret are required")
	}

	var expiresAt int64
	var approvedNodeID sql.NullString
	var caCertPEM sql.NullString
	var certPEM sql.NullString
	var deniedAt sql.NullInt64
	err := s.db.QueryRow(
		`SELECT expires_at, approved_node_id, ca_cert_pem, cert_pem, denied_at
		 FROM join_requests
		 WHERE id = ? AND secret_hash = ?`,
		id,
		hashToken(secret),
	).Scan(&expiresAt, &approvedNodeID, &caCertPEM, &certPEM, &deniedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("join request not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load join request: %w", err)
	}
	if deniedAt.Valid {
		return &RequestStatus{Status: "denied"}, nil
	}
	if expiresAt <= time.Now().Unix() {
		return &RequestStatus{Status: "expired"}, nil
	}
	if !approvedNodeID.Valid {
		return &RequestStatus{Status: "pending"}, nil
	}
	if !caCertPEM.Valid || !certPEM.Valid {
		return nil, fmt.Errorf("approved join request is missing certificate data")
	}

	return &RequestStatus{
		Status: "approved",
		Result: &JoinResult{
			NodeID:    approvedNodeID.String,
			PanelAddr: s.panelAddr,
			CACertPEM: caCertPEM.String,
			CertPEM:   certPEM.String,
		},
	}, nil
}

func (s *Service) ApproveRequest(id, approvedBy string) (*JoinResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("join request id is required")
	}

	var nodeName sql.NullString
	var csrPEM string
	var expiresAt int64
	var approvedAt sql.NullInt64
	var deniedAt sql.NullInt64
	err := s.db.QueryRow(
		`SELECT node_name, csr_pem, expires_at, approved_at, denied_at
		 FROM join_requests
		 WHERE id = ?`,
		id,
	).Scan(&nodeName, &csrPEM, &expiresAt, &approvedAt, &deniedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("join request not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load join request: %w", err)
	}
	if approvedAt.Valid {
		return nil, fmt.Errorf("join request already approved")
	}
	if deniedAt.Valid {
		return nil, fmt.Errorf("join request denied")
	}
	if expiresAt <= time.Now().Unix() {
		return nil, fmt.Errorf("join request expired")
	}

	csr, err := parseCSR(csrPEM)
	if err != nil {
		return nil, err
	}
	nodeID, err := newNodeID()
	if err != nil {
		return nil, err
	}
	caCertPEM, caCert, caKey, err := s.loadCA()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCAUnavailable, err)
	}
	certPEM, fingerprint, subject, err := signAgentCSR(csr, caCert, caKey, nodeID)
	if err != nil {
		return nil, err
	}

	name := ""
	if nodeName.Valid {
		name = nodeName.String
	}
	now := time.Now().Unix()
	_, err = s.db.Exec(
		`INSERT INTO nodes (id, name, last_seen, cert_fingerprint, cert_subject, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		nodeID,
		name,
		now,
		fingerprint,
		subject,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	_, err = s.db.Exec(
		`UPDATE join_requests
		 SET approved_by = ?, approved_node_id = ?, ca_cert_pem = ?, cert_pem = ?, approved_at = ?
		 WHERE id = ?`,
		approvedBy,
		nodeID,
		string(caCertPEM),
		string(certPEM),
		now,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to approve join request: %w", err)
	}

	return &JoinResult{
		NodeID:    nodeID,
		PanelAddr: s.panelAddr,
		CACertPEM: string(caCertPEM),
		CertPEM:   string(certPEM),
	}, nil
}

func (s *Service) ReviewRequest(id string) (*Review, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("join request id is required")
	}

	var nodeName sql.NullString
	var code string
	var expiresAt int64
	var approvedAt sql.NullInt64
	var deniedAt sql.NullInt64
	err := s.db.QueryRow(
		`SELECT node_name, verification_code, expires_at, approved_at, denied_at
		 FROM join_requests
		 WHERE id = ?`,
		id,
	).Scan(&nodeName, &code, &expiresAt, &approvedAt, &deniedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("join request not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load join request: %w", err)
	}

	status := "pending"
	if deniedAt.Valid {
		status = "denied"
	} else if approvedAt.Valid {
		status = "approved"
	} else if expiresAt <= time.Now().Unix() {
		status = "expired"
	}

	review := &Review{
		ID:               id,
		VerificationCode: code,
		ExpiresAt:        expiresAt,
		Status:           status,
	}
	if nodeName.Valid {
		review.NodeName = nodeName.String
	}

	return review, nil
}

func (s *Service) consumeToken(token string) (string, string, error) {
	now := time.Now().Unix()
	tokenHash := hashToken(token)

	var id string
	var nodeName sql.NullString
	var expiresAt int64
	var usedAt sql.NullInt64
	var revokedAt sql.NullInt64
	err := s.db.QueryRow(
		"SELECT id, node_name, expires_at, used_at, revoked_at FROM join_tokens WHERE token_hash = ?",
		tokenHash,
	).Scan(&id, &nodeName, &expiresAt, &usedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("invalid join token")
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to load join token: %w", err)
	}
	if usedAt.Valid {
		return "", "", fmt.Errorf("join token already used")
	}
	if revokedAt.Valid {
		return "", "", fmt.Errorf("join token revoked")
	}
	if expiresAt <= now {
		return "", "", fmt.Errorf("join token expired")
	}

	result, err := s.db.Exec(
		"UPDATE join_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL",
		now,
		id,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to consume join token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", "", fmt.Errorf("failed to confirm join token use: %w", err)
	}
	if rowsAffected != 1 {
		return "", "", fmt.Errorf("join token already used")
	}

	if nodeName.Valid {
		return id, nodeName.String, nil
	}

	return id, "", nil
}

func (s *Service) loadCA() ([]byte, *x509.Certificate, any, error) {
	certPEM, err := os.ReadFile(s.caCertPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read ca cert: %w", err)
	}
	keyPEM, err := os.ReadFile(s.caKeyPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read ca key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, nil, fmt.Errorf("failed to parse ca cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse ca cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, nil, fmt.Errorf("failed to parse ca key PEM")
	}
	key, err := parsePrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, nil, err
	}

	return certPEM, cert, key, nil
}

func parseCSR(csrPEM string) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("failed to parse CSR PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("invalid CSR signature: %w", err)
	}
	return csr, nil
}

func signAgentCSR(csr *x509.CertificateRequest, caCert *x509.Certificate, caKey any, nodeID string) ([]byte, string, string, error) {
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create certificate serial: %w", err)
	}

	nodeURI, err := url.Parse("deft:node:" + nodeID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create node URI: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: nodeID,
		},
		URIs:        []*url.URL{nodeURI},
		NotBefore:   now.Add(-1 * time.Minute),
		NotAfter:    now.Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, csr.PublicKey, caKey)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to sign node certificate: %w", err)
	}

	fingerprintBytes := sha256.Sum256(der)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if certPEM == nil {
		return nil, "", "", fmt.Errorf("failed to encode certificate PEM")
	}

	return certPEM, fingerprint, template.Subject.String(), nil
}

func parsePrivateKey(der []byte) (any, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("unsupported ca private key format")
}

func newNodeID() (string, error) {
	value, err := randomHex(12)
	if err != nil {
		return "", err
	}
	return nodeIDPrefix + value, nil
}

func randomHex(byteCount int) (string, error) {
	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to create random value: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func verificationCode() (string, error) {
	value, err := randomHex(4)
	if err != nil {
		return "", err
	}
	value = strings.ToUpper(value)
	return value[:4] + "-" + value[4:], nil
}

func tokenStatus(expiresAt int64, used, revoked bool, now int64) string {
	switch {
	case used:
		return "used"
	case revoked:
		return "revoked"
	case expiresAt <= now:
		return "expired"
	default:
		return "active"
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
