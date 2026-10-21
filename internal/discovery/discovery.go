// 文件说明：封装基于 etcd 的服务实例注册、状态更新和 ready 实例查询。
package discovery

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"sync"
	"time"

	"game-shelf/internal/config"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Instance struct {
	ServiceName  string `json:"service_name"`
	InstanceID   string `json:"instance_id"`
	Env          string `json:"env"`
	Version      string `json:"version"`
	Zone         string `json:"zone"`
	Gray         bool   `json:"gray"`
	Weight       int    `json:"weight"`
	Status       string `json:"status"`
	InternalAddr string `json:"internal_addr"`
	PublicAddr   string `json:"public_addr"`
	MaxConn      int    `json:"max_conn"`
	CurrentConn  int64  `json:"current_conn"`
	StartedAt    string `json:"started_at"`
}

type Registrar struct {
	client *clientv3.Client
	cfg    config.App
	key    string

	mu      sync.Mutex
	status  string
	current int64
	cancel  context.CancelFunc
	leaseID clientv3.LeaseID
	started time.Time
}

func NewClient(cfg config.Discovery) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   cfg.Etcd,
		DialTimeout: 3 * time.Second,
	})
}

func NewRegistrar(client *clientv3.Client, cfg config.App, status string) *Registrar {
	return &Registrar{
		client:  client,
		cfg:     cfg,
		key:     path.Join(cfg.Discovery.BasePath, cfg.Service.Name, cfg.Service.InstanceID),
		status:  status,
		started: time.Now().UTC(),
	}
}

func (r *Registrar) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	lease, err := r.client.Grant(runCtx, r.cfg.Discovery.LeaseTTLSeconds)
	if err != nil {
		cancel()
		return err
	}
	r.leaseID = lease.ID
	if err := r.putLocked(runCtx); err != nil {
		cancel()
		return err
	}

	ch, err := r.client.KeepAlive(runCtx, lease.ID)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		for range ch {
		}
	}()
	return nil
}

func (r *Registrar) Stop(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	if r.leaseID != 0 {
		_, _ = r.client.Revoke(ctx, r.leaseID)
	}
}

func (r *Registrar) Update(ctx context.Context, status string, currentConn int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
	r.current = currentConn
	if r.leaseID == 0 {
		return nil
	}
	return r.putLocked(ctx)
}

func (r *Registrar) putLocked(ctx context.Context) error {
	data, err := json.Marshal(r.instanceLocked())
	if err != nil {
		return err
	}
	_, err = r.client.Put(ctx, r.key, string(data), clientv3.WithLease(r.leaseID))
	return err
}

func (r *Registrar) instanceLocked() Instance {
	return Instance{
		ServiceName:  r.cfg.Service.Name,
		InstanceID:   r.cfg.Service.InstanceID,
		Env:          r.cfg.Service.Env,
		Version:      r.cfg.Service.Version,
		Zone:         r.cfg.Service.Zone,
		Gray:         r.cfg.Service.Gray,
		Weight:       r.cfg.Service.Weight,
		Status:       r.status,
		InternalAddr: r.cfg.Internal.AdvertiseAddr,
		PublicAddr:   r.cfg.Public.AdvertiseAddr,
		MaxConn:      r.cfg.Limit.MaxConn,
		CurrentConn:  r.current,
		StartedAt:    r.started.Format(time.RFC3339),
	}
}

func ListReady(ctx context.Context, client *clientv3.Client, basePath, serviceName string) ([]Instance, error) {
	resp, err := client.Get(ctx, path.Join(basePath, serviceName)+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	instances := make([]Instance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var inst Instance
		if err := json.Unmarshal(kv.Value, &inst); err != nil {
			continue
		}
		if strings.EqualFold(inst.Status, "ready") {
			instances = append(instances, inst)
		}
	}
	return instances, nil
}

func Check(ctx context.Context, client *clientv3.Client, cfg config.Discovery) error {
	endpoint := ""
	if len(cfg.Etcd) > 0 {
		endpoint = cfg.Etcd[0]
	}
	_, err := client.Status(ctx, endpoint)
	return err
}
