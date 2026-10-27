// 文件说明：在世界锁外经发现访问db；后台提交失败保留待存，查询不阻塞世界规则。
package main

import (
	"context"
	"errors"
	"game-shelf/internal/discovery"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"
	"time"
)

func (s *LobbyService) scoreCall(ctx context.Context, req *contracts.ScoresRequest, save bool) (*contracts.ScoresReply, error) {
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	instances, err := discovery.ListReady(callCtx, s.etcd, s.app.Discovery.BasePath, "db")
	if err != nil {
		return nil, err
	}
	if len(instances) != 1 {
		return nil, errors.New("single_db_unavailable")
	}
	client := transport.NewClient(instances[0].InternalAddr, 2*time.Second, s.app.Limit.MaxBodyBytes, s.log)
	defer client.Close()
	requestID, responseID := contracts.LBDB_QueryScoresReq, contracts.DBLB_QueryScoresResp
	if save {
		requestID, responseID = contracts.LBDB_SaveScoreReq, contracts.DBLB_SaveScoreResp
	}
	var reply contracts.ScoresReply
	if err = client.Call(callCtx, requestID, responseID, req, &reply); err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, errors.New(reply.Error)
	}
	return &reply, nil
}
func (s *LobbyService) submitResult(ctx context.Context) {
	record := s.world.ClaimResult(time.Now())
	if record == nil {
		return
	}
	_, err := s.scoreCall(ctx, &contracts.ScoresRequest{Record: record}, true)
	s.world.CompleteResult(record.RoundID, err == nil, time.Now())
	if err != nil {
		s.log.Error("score pending retry", "round_id", record.RoundID, "error", err)
	}
}
