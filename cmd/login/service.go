// 文件说明：严格验证账号口令后才签发网关凭据，注册和登录共享限频与依赖边界。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"game-shelf/client"
	"game-shelf/internal/config"
	"game-shelf/internal/credentials"
	"game-shelf/internal/discovery"
	"game-shelf/internal/session"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"
	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"
)

type LoginHTTP struct {
	app       config.App
	log       *slog.Logger
	redis     *redis.Client
	etcd      *clientv3.Client
	store     *session.Store
	ctx       context.Context
	mu        sync.Mutex
	dbClients map[string]*transport.Client
}

func (h *LoginHTTP) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) { h.authenticate(w, r, false) })
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) { h.authenticate(w, r, true) })
	mux.Handle("/", client.Handler())
	return mux
}
func (h *LoginHTTP) authenticate(w http.ResponseWriter, r *http.Request, register bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != scheme+"://"+r.Host {
		writeJSON(w, 403, map[string]string{"error": "untrusted_origin"})
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeJSON(w, 415, map[string]string{"error": "json_required"})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
	if err != nil || !utf8.Valid(data) {
		writeJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	var req contracts.LoginHTTPRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	key, _, err := credentials.Normalize(req.Account)
	if err != nil || len(req.Channel) > 32 {
		writeJSON(w, 400, map[string]string{"error": "invalid_account"})
		return
	}
	if credentials.ValidatePassword(req.Password) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_password"})
		return
	}
	if h.redis == nil || h.etcd == nil || h.store == nil {
		writeJSON(w, 503, map[string]string{"error": "dependency_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	allowed, err := h.allowAuth(ctx, r.RemoteAddr, key)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "dependency_unavailable"})
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, 429, map[string]string{"error": "auth_rate_limited"})
		return
	}
	gate, err := h.selectGate(ctx)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "no_gate_available"})
		return
	}
	account, err := h.verifyAccount(ctx, req, register)
	if err != nil {
		h.log.Error("account service unavailable", "error", err)
		writeJSON(w, 503, map[string]string{"error": "db_unavailable"})
		return
	}
	switch account.Code {
	case contracts.AccountInvalid:
		writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
		return
	case contracts.AccountExists:
		writeJSON(w, 409, map[string]string{"error": "account_exists"})
		return
	case contracts.AccountBusy:
		w.Header().Set("Retry-After", "1")
		writeJSON(w, 429, map[string]string{"error": "auth_busy"})
		return
	case contracts.AccountOK:
	default:
		writeJSON(w, 503, map[string]string{"error": "db_unavailable"})
		return
	}
	if account.UserID == "" || account.Account == "" {
		writeJSON(w, 503, map[string]string{"error": "db_unavailable"})
		return
	}
	token, err := h.store.CreateLoginToken(ctx, account.UserID, account.Account, req.Channel, gate.InstanceID, h.app.LoginTokenTTL())
	if err != nil {
		h.log.Error("create login token failed", "error", err)
		writeJSON(w, 503, map[string]string{"error": "create_token_failed"})
		return
	}
	code := http.StatusOK
	if register {
		code = http.StatusCreated
	}
	writeJSON(w, code, contracts.LoginHTTPReply{UserID: account.UserID, Token: token.Token, GateInstanceID: gate.InstanceID, GateAddr: gate.PublicAddr, GateWebSocketURL: gate.WebSocketURL, ExpireAt: token.ExpiresAt.Format(time.RFC3339)})
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
func (h *LoginHTTP) verifyAccount(ctx context.Context, req contracts.LoginHTTPRequest, register bool) (contracts.AccountReply, error) {
	instances, err := discovery.ListReady(ctx, h.etcd, h.app.Discovery.BasePath, "db")
	if err != nil {
		return contracts.AccountReply{}, err
	}
	if len(instances) == 0 {
		return contracts.AccountReply{}, redis.Nil
	}
	requestID, responseID := contracts.LGDB_VerifyAccountReq, contracts.DBLG_VerifyAccountResp
	if register {
		requestID, responseID = contracts.LGDB_RegisterAccountReq, contracts.DBLG_RegisterAccountResp
	}
	var reply contracts.AccountReply
	err = h.dbClient(instances[0]).Call(ctx, requestID, responseID, &contracts.AccountRequest{Account: req.Account, Password: req.Password}, &reply)
	return reply, err
}
func (h *LoginHTTP) dbClient(instance discovery.Instance) *transport.Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dbClients == nil {
		h.dbClients = map[string]*transport.Client{}
	}
	if c := h.dbClients[instance.InstanceID]; c != nil {
		return c
	}
	c := transport.NewClient(instance.InternalAddr, 3*time.Second, h.app.Limit.MaxBodyBytes, h.log)
	c.StartHeartbeat(h.ctx, contracts.LGDB_PingReq, contracts.DBLG_PingResp, h.app.InternalHeartbeatInterval())
	h.dbClients[instance.InstanceID] = c
	return c
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
