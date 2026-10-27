// 文件说明：lobby通过发现与内部TCP访问db收藏存档，不直接连接MongoDB。
package main

import (
	"context"
	"errors"
	"game-shelf/internal/discovery"
	"game-shelf/internal/transport"
	"game-shelf/pkg/contracts"
	"time"
)

func (s *LobbyService) Load(ctx context.Context, userID string) (*contracts.Journal, error) {
	return s.callBook(ctx, contracts.LBDB_LoadBookReq, contracts.DBLB_LoadBookResp, &contracts.BookRequest{UserID: userID})
}
func (s *LobbyService) Save(ctx context.Context, userID string, next *contracts.Journal) (*contracts.Journal, error) {
	return s.callBook(ctx, contracts.LBDB_SaveBookReq, contracts.DBLB_SaveBookResp, &contracts.BookRequest{UserID: userID, Journal: next})
}
func (s *LobbyService) callBook(ctx context.Context, requestID, responseID uint32, req *contracts.BookRequest) (*contracts.Journal, error) {
	instances, err := discovery.ListReady(ctx, s.etcd, s.app.Discovery.BasePath, "db")
	if err != nil {
		return nil, err
	}
	if len(instances) != 1 {
		return nil, errors.New("single_book_store_unavailable")
	}
	instance := instances[0]
	key := instance.InstanceID + "@" + instance.InternalAddr
	s.bookMu.Lock()
	if s.bookClients == nil {
		s.bookClients = make(map[string]*transport.Client)
	}
	client := s.bookClients[key]
	if client == nil {
		client = transport.NewClient(instance.InternalAddr, 2*time.Second, s.app.Limit.MaxBodyBytes, s.log)
		client.StartHeartbeat(s.ctx, contracts.LBDB_PingReq, contracts.DBLB_PingResp, s.app.InternalHeartbeatInterval())
		s.bookClients[key] = client
	}
	s.bookMu.Unlock()
	var reply contracts.BookReply
	if err := client.Call(ctx, requestID, responseID, req, &reply); err != nil {
		return nil, err
	}
	if reply.Error != "" || reply.Journal == nil {
		return nil, errors.New("progress_unavailable")
	}
	return reply.Journal, nil
}
func (s *LobbyService) closeBookClients() {
	s.bookMu.Lock()
	defer s.bookMu.Unlock()
	for _, client := range s.bookClients {
		_ = client.Close()
	}
}
