// 文件说明：实现 login 公网 HTTP 登录入口和 gate 选择逻辑。
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"game-shelf/client"
	"game-shelf/internal/config"
	"game-shelf/internal/discovery"
	"game-shelf/internal/session"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type LoginHTTP struct {
	app   config.App
	log   *slog.Logger
	redis *redis.Client
	etcd  *clientv3.Client
	store *session.Store
	ctx   context.Context

	mu        sync.Mutex
	dbClients map[string]*transport.Client
}

func (h *LoginHTTP) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", h.login)
	mux.Handle("/", client.Handler())
	return mux
}

func (h *LoginHTTP) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	if h.redis == nil || h.etcd == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "dependency_unavailable"})
		return
	}

	var req contracts.LoginHTTPRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}

	account := strings.TrimSpace(req.Account)
	if account == "" {
		account = "guest"
	}
	if !utf8.ValidString(account) || utf8.RuneCountInString(account) > 24 || len(req.Channel) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_account"})
		return
	}
	userID := stableUserID(account, req.Channel)

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	gate, err := h.selectGate(ctx)
	if err != nil {
		h.log.Error("select gate failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no_gate_available"})
		return
	}
	if err := h.upsertPlayer(ctx, userID, account, req.Channel); err != nil {
		h.log.Error("upsert player failed", "error", err, "user_id", userID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "db_unavailable"})
		return
	}

	token, err := h.store.CreateLoginToken(ctx, userID, account, req.Channel, gate.InstanceID, h.app.LoginTokenTTL())
	if err != nil {
		h.log.Error("create login token failed", "error", err, "user_id", userID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create_token_failed"})
		return
	}

	writeJSON(w, http.StatusOK, contracts.LoginHTTPReply{
		UserID:           userID,
		Token:            token.Token,
		GateInstanceID:   gate.InstanceID,
		GateAddr:         gate.PublicAddr,
		GateWebSocketURL: gate.WebSocketURL,
		ExpireAt:         token.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *LoginHTTP) selectGate(ctx context.Context) (discovery.Instance, error) {
	gates, err := discovery.ListReady(ctx, h.etcd, h.app.Discovery.BasePath, "gate")
	if err != nil {
		return discovery.Instance{}, err
	}
	if len(gates) == 0 {
		return discovery.Instance{}, redis.Nil
	}
	return gates[0], nil
}

func (h *LoginHTTP) upsertPlayer(ctx context.Context, userID, account, channel string) error {
	dbs, err := discovery.ListReady(ctx, h.etcd, h.app.Discovery.BasePath, "db")
	if err != nil {
		return err
	}
	if len(dbs) == 0 {
		return redis.Nil
	}
	client := h.dbClient(dbs[0])

	var reply contracts.UpsertPlayerResp
	return client.Call(ctx,
		contracts.LGDB_UpsertPlayerReq,
		contracts.DBLG_UpsertPlayerResp,
		&contracts.UpsertPlayerReq{UserID: userID, Account: account, Channel: channel},
		&reply,
	)
}

func (h *LoginHTTP) dbClient(instance discovery.Instance) *transport.Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dbClients == nil {
		h.dbClients = map[string]*transport.Client{}
	}
	if client := h.dbClients[instance.InstanceID]; client != nil {
		return client
	}
	client := transport.NewClient(instance.InternalAddr, 3*time.Second, h.app.Limit.MaxBodyBytes, h.log)
	client.StartHeartbeat(h.ctx, contracts.LGDB_PingReq, contracts.DBLG_PingResp, h.app.InternalHeartbeatInterval())
	h.dbClients[instance.InstanceID] = client
	return client
}

func stableUserID(account, channel string) string {
	sum := sha256.Sum256([]byte(channel + ":" + account))
	return hex.EncodeToString(sum[:8])
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
