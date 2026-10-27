// 文件说明：仅内部lobby可提交权威成绩，写失败不返回已保存，不对浏览器暴露任意文档入口。
package main

import (
	"context"
	"game-shelf/internal/protocol"
	"game-shelf/pkg/contracts"
	"time"
)

func (s *DBService) handleScores(ctx context.Context, msg protocol.Message) (protocol.Message, bool) {
	id := contracts.DBLB_QueryScoresResp
	if msg.ProtoID == contracts.LBDB_SaveScoreReq {
		id = contracts.DBLB_SaveScoreResp
	}
	reply := &contracts.ScoresReply{}
	var req contracts.ScoresRequest
	if protocol.UnmarshalBody(msg, &req) != nil {
		reply.Error = "invalid_score_request"
	} else {
		callCtx, cancel := context.WithTimeout(ctx, 1800*time.Millisecond)
		defer cancel()
		var err error
		if msg.ProtoID == contracts.LBDB_SaveScoreReq {
			err = s.scores.Save(callCtx, req.Record)
		} else {
			reply.Board, err = s.scores.Query(callCtx, req.UserID, req.Mode, req.Day)
		}
		if err != nil {
			reply.Error = "scores_unavailable"
			s.log.Error("score storage unavailable", "error", err)
		}
	}
	response, err := protocol.MarshalMessage(id, msg.Seq, reply)
	return response, err == nil
}
