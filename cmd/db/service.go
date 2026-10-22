// 文件说明：db 只接受有口令的注册/验证，旧无密码 upsert 入口已撤销。
package main

import (
	"context"
	"errors"
	"game-shelf/internal/config"
	"game-shelf/internal/credentials"
	"game-shelf/internal/protocol"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"
	"log/slog"
	"time"
)

type DBService struct {
	app      config.App
	log      *slog.Logger
	accounts *credentials.Store
}

func (s *DBService) Handle(ctx context.Context, _ *transport.Conn, msg protocol.Message) (protocol.Message, bool) {
	switch msg.ProtoID {
	case contracts.LGDB_PingReq:
		return pingReply(contracts.DBLG_PingResp, msg.Seq), true
	case contracts.LGDB_RegisterAccountReq, contracts.LGDB_VerifyAccountReq:
		var req contracts.AccountRequest
		if protocol.UnmarshalBody(msg, &req) != nil {
			return errorReply(contracts.SYS_ErrorNotify, msg.Seq, "invalid_account_request"), true
		}
		var account credentials.Account
		var err error
		responseID := contracts.DBLG_VerifyAccountResp
		if msg.ProtoID == contracts.LGDB_RegisterAccountReq {
			responseID = contracts.DBLG_RegisterAccountResp
			account, err = s.accounts.Register(ctx, req.Account, req.Password)
		} else {
			account, err = s.accounts.Login(ctx, req.Account, req.Password)
		}
		reply := contracts.AccountReply{Code: contracts.AccountOK}
		switch {
		case err == nil:
			reply.UserID = account.UserID
			reply.Account = account.Name
		case errors.Is(err, credentials.ErrAccountExists):
			reply.Code = contracts.AccountExists
		case errors.Is(err, credentials.ErrBusy):
			reply.Code = contracts.AccountBusy
		case errors.Is(err, credentials.ErrInvalidCredentials), errors.Is(err, credentials.ErrInvalidAccount), errors.Is(err, credentials.ErrInvalidPassword):
			reply.Code = contracts.AccountInvalid
		default:
			reply.Code = contracts.AccountUnavailable
			s.log.Error("account storage unavailable", "error", err)
		}
		response, marshalErr := protocol.MarshalMessage(responseID, msg.Seq, &reply)
		return response, marshalErr == nil
	default:
		return errorReply(contracts.SYS_ErrorNotify, msg.Seq, "unknown_proto"), true
	}
}
func pingReply(protoID, seq uint32) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.PingResp{UnixMillis: time.Now().UnixMilli()})
	return resp
}
func errorReply(protoID, seq uint32, message string) protocol.Message {
	resp, _ := protocol.MarshalMessage(protoID, seq, &contracts.ErrorNotify{Code: 1, Message: message})
	return resp
}
