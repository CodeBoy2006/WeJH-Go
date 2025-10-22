package circuitBreaker

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"wejh-go/config/api/funnelApi"
	cbConfig "wejh-go/config/circuitBreaker"
	"wejh-go/config/redis"
)

// ---------------------------
// 对外暴露的负载均衡结构体与接口
// ---------------------------

type lbPicker interface {
	Add(host string)
	Remove(host string)
	HasAvailable() bool
	PickOne(ctx context.Context) (string, error)
	PickN(ctx context.Context, n int) ([]string, error)
	SumWeight(ctx context.Context) int
}

type LoadBalance struct {
	zfLB    lbPicker
	oauthLB lbPicker
	cfg     cbConfig.LoadBalanceConfig
}

// NewLoadBalance 构造带权重的 LB（SWRR 实现）
func NewLoadBalance(cfg cbConfig.LoadBalanceConfig) LoadBalance {
	return LoadBalance{
		zfLB:    newSwrrBalancer(cfg, funnelApi.ZF),
		oauthLB: newSwrrBalancer(cfg, funnelApi.Oauth),
		cfg:     cfg,
	}
}

// Add 将 host 加入某一登录方式的池
func (l *LoadBalance) Add(host string, loginType funnelApi.LoginType) {
	switch loginType {
	case funnelApi.ZF:
		l.zfLB.Add(host)
	case funnelApi.Oauth:
		l.oauthLB.Add(host)
	}
}

// Remove 将 host 从某一登录方式的池移除（置为不可用）
func (l *LoadBalance) Remove(host string, loginType funnelApi.LoginType) {
	switch loginType {
	case funnelApi.ZF:
		l.zfLB.Remove(host)
	case funnelApi.Oauth:
		l.oauthLB.Remove(host)
	}
}

// Pick 选择一个 host 与登录方式
// 策略：若 zfFlag 与 oauthFlag 都可用，则按两侧"可用权重和"做比例选择；否则选择唯一可用侧
func (l *LoadBalance) Pick(zfFlag, oauthFlag bool) (string, funnelApi.LoginType, error) {
	ctx := context.Background()

	type choice struct {
		host      string
		loginType funnelApi.LoginType
		err       error
	}

	// 情形：只有一种密码可用（或只有一种侧有可用权重/可用节点）
	if zfFlag && !oauthFlag {
		h, err := l.zfLB.PickOne(ctx)
		return h, funnelApi.ZF, err
	}
	if oauthFlag && !zfFlag {
		h, err := l.oauthLB.PickOne(ctx)
		return h, funnelApi.Oauth, err
	}

	// 两种都可用时，根据权重总和决定选择哪边
	zfSum := l.zfLB.SumWeight(ctx)
	oaSum := l.oauthLB.SumWeight(ctx)

	// 若一边没有可用权重，退化为另一边
	if zfSum <= 0 && oaSum > 0 {
		h, err := l.oauthLB.PickOne(ctx)
		return h, funnelApi.Oauth, err
	}
	if oaSum <= 0 && zfSum > 0 {
		h, err := l.zfLB.PickOne(ctx)
		return h, funnelApi.ZF, err
	}
	if zfSum <= 0 && oaSum <= 0 {
		return "", "", errors.New("no available upstream")
	}

	// 概率选择
	total := zfSum + oaSum
	r := rand.Intn(total)
	if r < oaSum {
		h, err := l.oauthLB.PickOne(ctx)
		return h, funnelApi.Oauth, err
	}
	h, err := l.zfLB.PickOne(ctx)
	return h, funnelApi.ZF, err
}

// PickN 选择同一登录方式下 n 个"互不相同"的 host（用于对冲）
func (l *LoadBalance) PickN(zfFlag, oauthFlag bool, n int) ([]string, funnelApi.LoginType, error) {
	ctx := context.Background()
	if n <= 0 {
		n = 1
	}
	if zfFlag && !oauthFlag {
		h, err := l.zfLB.PickN(ctx, n)
		return h, funnelApi.ZF, err
	}
	if oauthFlag && !zfFlag {
		h, err := l.oauthLB.PickN(ctx, n)
		return h, funnelApi.Oauth, err
	}
	// 两侧都可用：按总权重比例决定选哪边，然后在对应池中 PickN
	zfSum := l.zfLB.SumWeight(ctx)
	oaSum := l.oauthLB.SumWeight(ctx)
	if zfSum <= 0 && oaSum > 0 {
		h, err := l.oauthLB.PickN(ctx, n)
		return h, funnelApi.Oauth, err
	}
	if oaSum <= 0 && zfSum > 0 {
		h, err := l.zfLB.PickN(ctx, n)
		return h, funnelApi.ZF, err
	}
	if zfSum <= 0 && oaSum <= 0 {
		return nil, "", errors.New("no available upstream")
	}
	total := zfSum + oaSum
	r := rand.Intn(total)
	if r < oaSum {
		h, err := l.oauthLB.PickN(ctx, n)
		return h, funnelApi.Oauth, err
	}
	h, err := l.zfLB.PickN(ctx, n)
	return h, funnelApi.ZF, err
}

// ---------------------------
// SWRR 加权轮询 + 权重来源（配置/Redis/时间窗）
// ---------------------------

type swrrNode struct {
	host        string
	current     int
	available   bool
	lastRefresh time.Time // 本地权重缓存刷新时间
}

type swrrBalancer struct {
	mu        sync.Mutex
	nodes     map[string]*swrrNode
	order     []*swrrNode
	loginType funnelApi.LoginType
	cfg       cbConfig.LoadBalanceConfig
}

func newSwrrBalancer(cfg cbConfig.LoadBalanceConfig, lt funnelApi.LoginType) *swrrBalancer {
	return &swrrBalancer{
		nodes:     make(map[string]*swrrNode),
		order:     make([]*swrrNode, 0, 8),
		loginType: lt,
		cfg:       cfg,
	}
}

func (b *swrrBalancer) Add(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}
	if n, ok := b.nodes[host]; ok {
		n.available = true
		return
	}
	n := &swrrNode{
		host:      host,
		available: true,
	}
	b.nodes[host] = n
	b.order = append(b.order, n)
}

func (b *swrrBalancer) Remove(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n, ok := b.nodes[host]; ok {
		n.available = false
	}
}

func (b *swrrBalancer) HasAvailable() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, n := range b.order {
		if n.available && b.effectiveWeightNoLock(n) > 0 {
			return true
		}
	}
	return false
}

func (b *swrrBalancer) SumWeight(ctx context.Context) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	sum := 0
	for _, n := range b.order {
		if !n.available {
			continue
		}
		w := b.effectiveWeightNoLock(n)
		if w > 0 {
			sum += w
		}
	}
	return sum
}

func (b *swrrBalancer) PickOne(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var best *swrrNode
	total := 0

	for _, n := range b.order {
		if !n.available {
			continue
		}
		w := b.effectiveWeightNoLock(n)
		if w <= 0 {
			continue
		}
		n.current += w
		total += w
		if best == nil || n.current > best.current {
			best = n
		}
	}
	if best == nil {
		return "", errors.New("no available upstream in this loginType")
	}
	best.current -= total
	return best.host, nil
}

func (b *swrrBalancer) PickN(ctx context.Context, n int) ([]string, error) {
	if n <= 0 {
		n = 1
	}
	// 为避免互相影响多次 Pick，复制一份 current 做临时轮询
	type tmpNode struct {
		node    *swrrNode
		current int
		weight  int
	}
	b.mu.Lock()
	tmp := make([]*tmpNode, 0, len(b.order))
	for _, n0 := range b.order {
		if !n0.available {
			continue
		}
		w := b.effectiveWeightNoLock(n0)
		if w <= 0 {
			continue
		}
		tmp = append(tmp, &tmpNode{
			node:    n0,
			current: n0.current,
			weight:  w,
		})
	}
	b.mu.Unlock()

	res := make([]string, 0, n)
	if len(tmp) == 0 {
		return nil, errors.New("no available upstream in this loginType")
	}
	// 平滑加权轮询选 N 个互不相同的主机
	for i := 0; i < n && len(res) < len(tmp); i++ {
		var best *tmpNode
		total := 0
		for _, t := range tmp {
			t.current += t.weight
			total += t.weight
			if best == nil || t.current > best.current {
				best = t
			}
		}
		if best == nil {
			break
		}
		best.current -= total
		res = append(res, best.node.host)
	}
	return res, nil
}

// effectiveWeightNoLock 计算某节点在当前时间点的有效权重：配置兜底 -> 时间段覆盖 -> Redis 动态
func (b *swrrBalancer) effectiveWeightNoLock(n *swrrNode) int {
	now := time.Now()
	// 本地缓存：避免每次 pick 都 hit Redis
	ttl := b.cfg.Redis.CacheTTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	needRefresh := n.lastRefresh.IsZero() || now.Sub(n.lastRefresh) >= ttl

	// 1) 配置兜底
	base := b.cfg.DefaultWeight
	if base <= 0 {
		base = 100
	}
	if w, ok := b.cfg.Weights[n.host]; ok {
		if b.loginType == funnelApi.ZF && w.ZF > 0 {
			base = w.ZF
		}
		if b.loginType == funnelApi.Oauth && w.Oauth > 0 {
			base = w.Oauth
		}
	}

	// 2) 时间段覆盖（可直接置 0）
	w := b.applyWindowOverrideNoLock(n.host, base, now)

	// 3) Redis 覆盖（仅当需要刷新）
	if needRefresh {
		if rw, ok := b.getWeightFromRedisNoLock(n.host); ok {
			w = rw
		}
		n.lastRefresh = now
	}
	if w < 0 {
		w = 0
	}
	if w > 100 {
		w = 100
	}
	return w
}

func (b *swrrBalancer) applyWindowOverrideNoLock(host string, base int, now time.Time) int {
	if len(b.cfg.Windows) == 0 {
		return base
	}
	hhmm := now.Format("15:04")
	for _, win := range b.cfg.Windows {
		if inTimeRange(hhmm, win.Time) {
			if ww, ok := win.Overrides[host]; ok {
				if b.loginType == funnelApi.ZF && ww.ZF >= 0 {
					return ww.ZF
				}
				if b.loginType == funnelApi.Oauth && ww.Oauth >= 0 {
					return ww.Oauth
				}
			}
		}
	}
	return base
}

func inTimeRange(cur, win string) bool {
	// win 形如 "00:30-06:00"
	parts := strings.Split(win, "-")
	if len(parts) != 2 {
		return false
	}
	start, end := parts[0], parts[1]
	if start <= end {
		return cur >= start && cur <= end
	}
	// 跨天的时间段
	return cur >= start || cur <= end
}

func (b *swrrBalancer) getWeightFromRedisNoLock(host string) (int, bool) {
	if redis.RedisClient == nil {
		return 0, false
	}
	key := b.cfg.Redis.WeightPrefix
	if key == "" {
		key = "lb:weight"
	}
	var lt string
	if b.loginType == funnelApi.ZF {
		lt = "ZF"
	} else {
		lt = "OAUTH"
	}
	// lb:weight:{loginType}:{host}
	rk := key + ":" + lt + ":" + host
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	val, err := redis.RedisClient.Get(ctx, rk).Result()
	if err != nil || val == "" {
		return 0, false
	}
	// 简单解析为 0~100
	num := 0
	for i := 0; i < len(val); i++ {
		c := val[i]
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
			if num > 100 {
				num = 100
			}
		} else {
			break
		}
	}
	return num, true
}
