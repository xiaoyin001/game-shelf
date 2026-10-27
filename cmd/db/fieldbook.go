// 文件说明：提供lobby专用收藏读写协议，不向浏览器暴露MongoDB或任意文档写入。
package main

import (
	"context"
	"game-shelf/internal/protocol"
	"game-shelf/pkg/contracts"
	"time"
)

func (s *DBService) handleBook(ctx context.Context, msg protocol.Message) (protocol.Message, bool) {
	id := contracts.DBLB_LoadBookResp
	if msg.ProtoID == contracts.LBDB_SaveBookReq {
		id = contracts.DBLB_SaveBookResp
	}
	reply := &contracts.BookReply{}
	var req contracts.BookRequest
	if protocol.UnmarshalBody(msg, &req) != nil {
		reply.Error = "invalid_field_book"
	} else {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		var err error
		if msg.ProtoID == contracts.LBDB_LoadBookReq {
			reply.Journal, err = s.books.Load(callCtx, req.UserID)
		} else {
			reply.Journal, err = s.books.Save(callCtx, req.UserID, req.Journal)
		}
		if err != nil {
			reply.Error = "progress_unavailable"
			s.log.Error("field book storage unavailable", "error", err, "user_id", req.UserID)
		}
	}
	response, err := protocol.MarshalMessage(id, msg.Seq, reply)
	return response, err == nil
}
