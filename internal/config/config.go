// 文件说明：定义服务运行配置、默认值、校验和热更新轮询逻辑。
package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"time"
)

type App struct {
	// Service 描述当前进程的服务身份和实例调度信息。
	Service Service `json:"service"`
	// Public 描述暴露给客户端的公网监听和公布地址。
	Public Endpoint `json:"public"`
	// Internal 描述服务间 TCP 通信的内网监听和公布地址。
	Internal Endpoint `json:"internal"`
	// Health 描述健康检查 HTTP 监听配置。
	Health Health `json:"health"`
	// Discovery 描述 etcd 服务注册发现配置。
	Discovery Discovery `json:"discovery"`
	// Dependency 描述依赖检查和不可用退出策略。
	Dependency Dependency `json:"dependency"`
	// Heartbeat 描述客户端和内部长连接心跳策略。
	Heartbeat Heartbeat `json:"heartbeat"`
	// Limit 描述协议包和连接数限制。
	Limit Limit `json:"limit"`
	// Session 描述登录 token 和在线会话生命周期。
	Session Session `json:"session"`
	// Shutdown 描述优雅下线策略。
	Shutdown Shutdown `json:"shutdown"`
	// Redis 描述 Redis 连接配置。
	Redis Redis `json:"redis"`
	// Mongo 描述 MongoDB 连接配置。
	Mongo Mongo `json:"mongo"`
	// Log 描述结构化日志输出配置。
	Log Log `json:"log"`
	// Config 描述配置热更新轮询行为。
	Config Runtime `json:"config"`
}

type Service struct {
	// Name 是服务名，用于日志、健康检查和服务发现路径。
	Name string `json:"name"`
	// InstanceID 是当前进程实例 ID，多开时必须唯一。
	InstanceID string `json:"instance_id"`
	// Env 是运行环境，例如 local、private 或 prod。
	Env string `json:"env"`
	// Version 是服务版本号，用于实例识别和排查。
	Version string `json:"version"`
	// Zone 是实例所在逻辑区域。
	Zone string `json:"zone"`
	// Gray 标记当前实例是否属于灰度实例。
	Gray bool `json:"gray"`
	// Weight 是服务发现负载均衡权重，小于等于 0 时使用 100。
	Weight int `json:"weight"`
}

type Endpoint struct {
	// ListenAddr 是本进程实际监听地址。
	ListenAddr string `json:"listen_addr"`
	// AdvertiseAddr 是注册给其他进程或返回给客户端的地址。
	AdvertiseAddr string `json:"advertise_addr"`
}

type Health struct {
	// ListenAddr 是健康检查 HTTP 服务监听地址。
	ListenAddr string `json:"listen_addr"`
}

type Discovery struct {
	// BasePath 是 etcd 中服务注册发现的根路径。
	BasePath string `json:"base_path"`
	// Etcd 是 etcd endpoint 列表。
	Etcd []string `json:"etcd"`
	// LeaseTTLSeconds 是实例注册 lease TTL。
	LeaseTTLSeconds int64 `json:"lease_ttl_seconds"`
}

type Dependency struct {
	// CheckIntervalMS 是依赖检查间隔，单位毫秒。
	CheckIntervalMS int `json:"check_interval_ms"`
	// StartupGraceMS 是启动期最多等待依赖就绪的时间，单位毫秒。
	StartupGraceMS int `json:"startup_grace_ms"`
	// UnavailableExitAfterMS 是运行期依赖持续不可用后退出进程的时间，单位毫秒。
	UnavailableExitAfterMS int `json:"unavailable_exit_after_ms"`
}

type Heartbeat struct {
	// ClientIntervalMS 是客户端连接心跳发送间隔，单位毫秒。
	ClientIntervalMS int `json:"client_interval_ms"`
	// ClientIdleTimeoutMS 是客户端连接最大空闲时间，单位毫秒。
	ClientIdleTimeoutMS int `json:"client_idle_timeout_ms"`
	// InternalIntervalMS 是内部服务连接心跳发送间隔，单位毫秒。
	InternalIntervalMS int `json:"internal_interval_ms"`
	// InternalIdleTimeoutMS 是内部服务连接最大空闲时间，单位毫秒。
	InternalIdleTimeoutMS int `json:"internal_idle_timeout_ms"`
}

type Limit struct {
	// MaxBodyBytes 是单个协议包 body 最大字节数。
	MaxBodyBytes int `json:"max_body_bytes"`
	// MaxConn 是服务实例最大连接数，用于注册发现和本地限制。
	MaxConn int `json:"max_conn"`
}

type Session struct {
	// LoginTokenTTLMS 是 login token 有效期，单位毫秒。
	LoginTokenTTLMS int `json:"login_token_ttl_ms"`
	// OnlineSessionTTLMS 是在线 session 续期 TTL，单位毫秒。
	OnlineSessionTTLMS int `json:"online_session_ttl_ms"`
	// ReconnectTTLMS 是断线重连窗口，单位毫秒。
	ReconnectTTLMS int `json:"reconnect_ttl_ms"`
	// MultiLogin 控制同账号是否允许多端同时在线。
	MultiLogin bool `json:"multi_login"`
}

type Shutdown struct {
	// TimeoutMS 是优雅下线最大等待时间，单位毫秒。
	TimeoutMS int `json:"timeout_ms"`
	// GateKickMessage 是 gate 停服或顶号时返回客户端的原因。
	GateKickMessage string `json:"gate_kick_message"`
}

type Redis struct {
	// Addr 是 Redis 服务地址。
	Addr string `json:"addr"`
	// Password 是 Redis 认证密码，空字符串表示不使用密码。
	Password string `json:"password"`
	// DB 是 Redis 逻辑库编号。
	DB int `json:"db"`
}

type Mongo struct {
	// URI 是 MongoDB 连接串。
	URI string `json:"uri"`
	// Database 是业务数据使用的 MongoDB 数据库名。
	Database string `json:"database"`
}

type Log struct {
	// Level 是日志级别，支持 debug、info、warn 和 error。
	Level string `json:"level"`
	// Output 是日志输出方式，支持 stdout 和 file。
	Output string `json:"output"`
	// FilePath 是 file 输出模式下的日志文件路径。
	FilePath string `json:"file_path"`
	// MaxSizeMB 是单个日志文件最大尺寸。
	MaxSizeMB int `json:"max_size_mb"`
	// MaxBackups 是日志滚动后保留文件数量。
	MaxBackups int `json:"max_backups"`
	// MaxAgeDays 是日志文件最长保留天数。
	MaxAgeDays int `json:"max_age_days"`
	// Compress 控制滚动日志是否压缩。
	Compress bool `json:"compress"`
}

type Runtime struct {
	// WatchIntervalMS 是配置热更新轮询间隔，单位毫秒。
	WatchIntervalMS int `json:"watch_interval_ms"`
}

func Load(path string) (App, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return App{}, "", err
	}

	var app App
	if err := json.Unmarshal(data, &app); err != nil {
		return App{}, "", err
	}
	if err := app.Validate(); err != nil {
		return App{}, "", err
	}

	sum := sha256.Sum256(data)
	return app, hex.EncodeToString(sum[:]), nil
}

func (a *App) Validate() error {
	if a.Service.Name == "" {
		return errors.New("service.name is required")
	}
	if a.Service.InstanceID == "" {
		a.Service.InstanceID = a.Service.Name + "-local-1"
	}
	if a.Service.Weight <= 0 {
		a.Service.Weight = 100
	}
	if a.Service.Version == "" {
		a.Service.Version = "dev"
	}
	if a.Service.Env == "" {
		a.Service.Env = "local"
	}
	if a.Service.Zone == "" {
		a.Service.Zone = "local"
	}
	if a.Internal.ListenAddr == "" {
		return errors.New("internal.listen_addr is required")
	}
	if a.Internal.AdvertiseAddr == "" {
		a.Internal.AdvertiseAddr = a.Internal.ListenAddr
	}
	if a.Health.ListenAddr == "" {
		return errors.New("health.listen_addr is required")
	}
	if a.Discovery.BasePath == "" {
		a.Discovery.BasePath = "/game-shelf/services"
	}
	if len(a.Discovery.Etcd) == 0 {
		return errors.New("discovery.etcd must contain at least one endpoint")
	}
	if a.Discovery.LeaseTTLSeconds <= 0 {
		a.Discovery.LeaseTTLSeconds = 10
	}
	if a.Log.Output == "" {
		a.Log.Output = "stdout"
	}
	if a.Limit.MaxBodyBytes <= 0 {
		a.Limit.MaxBodyBytes = 1024 * 1024
	}
	if a.Limit.MaxConn <= 0 {
		a.Limit.MaxConn = 5000
	}
	return nil
}

func (a App) CheckInterval() time.Duration {
	if a.Dependency.CheckIntervalMS <= 0 {
		return 2 * time.Second
	}
	return time.Duration(a.Dependency.CheckIntervalMS) * time.Millisecond
}

func (a App) StartupGrace() time.Duration {
	if a.Dependency.StartupGraceMS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(a.Dependency.StartupGraceMS) * time.Millisecond
}

func (a App) UnavailableExitAfter() time.Duration {
	if a.Dependency.UnavailableExitAfterMS <= 0 {
		return time.Minute
	}
	return time.Duration(a.Dependency.UnavailableExitAfterMS) * time.Millisecond
}

func (a App) InternalHeartbeatInterval() time.Duration {
	if a.Heartbeat.InternalIntervalMS <= 0 {
		return 15 * time.Second
	}
	return time.Duration(a.Heartbeat.InternalIntervalMS) * time.Millisecond
}

func (a App) InternalIdleTimeout() time.Duration {
	if a.Heartbeat.InternalIdleTimeoutMS <= 0 {
		return 45 * time.Second
	}
	return time.Duration(a.Heartbeat.InternalIdleTimeoutMS) * time.Millisecond
}

func (a App) ClientIdleTimeout() time.Duration {
	if a.Heartbeat.ClientIdleTimeoutMS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(a.Heartbeat.ClientIdleTimeoutMS) * time.Millisecond
}

func (a App) LoginTokenTTL() time.Duration {
	if a.Session.LoginTokenTTLMS <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(a.Session.LoginTokenTTLMS) * time.Millisecond
}

func (a App) OnlineSessionTTL() time.Duration {
	if a.Session.OnlineSessionTTLMS <= 0 {
		return 90 * time.Second
	}
	return time.Duration(a.Session.OnlineSessionTTLMS) * time.Millisecond
}

func (a App) ReconnectTTL() time.Duration {
	if a.Session.ReconnectTTLMS <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(a.Session.ReconnectTTLMS) * time.Millisecond
}

func (a App) ShutdownTimeout() time.Duration {
	if a.Shutdown.TimeoutMS <= 0 {
		return 10 * time.Second
	}
	return time.Duration(a.Shutdown.TimeoutMS) * time.Millisecond
}

func (a App) WatchInterval() time.Duration {
	if a.Config.WatchIntervalMS <= 0 {
		return 2 * time.Second
	}
	return time.Duration(a.Config.WatchIntervalMS) * time.Millisecond
}

func Watch(ctx context.Context, path string, interval time.Duration, log *slog.Logger, onChange func(App, string)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	_, lastHash, err := Load(path)
	if err != nil {
		log.Error("initial config watch load failed", "path", path, "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next, hash, err := Load(path)
			if err != nil {
				log.Error("config reload rejected", "path", path, "error", err)
				continue
			}
			if hash == lastHash {
				continue
			}
			lastHash = hash
			onChange(next, hash)
		}
	}
}
