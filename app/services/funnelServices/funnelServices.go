package funnelServices

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"

	"wejh-go/app/apiException"
	"wejh-go/app/utils/circuitBreaker"
	"wejh-go/app/utils/fetch"
	"wejh-go/config/api/funnelApi"
)

// FunnelResponse 后端统一响应格式
type FunnelResponse struct {
	Code int         `json:"code" binding:"required"`
	Msg  string      `json:"message" binding:"required"`
	Data interface{} `json:"data"`
}

// 对单个后端节点做一次「带 413 重试」的调用
func singleHostRequest(ctx context.Context, host string, api funnelApi.FunnelApi, form url.Values) (FunnelResponse, error) {
	f := fetch.Fetch{}
	f.Init()

	var rc FunnelResponse
	var res []byte
	var err error

	// 保留原来最多 5 次、413 重试的语义
	for i := 0; i < 5; i++ {
		// 已经被上层取消，则直接退出
		select {
		case <-ctx.Done():
			return FunnelResponse{}, ctx.Err()
		default:
		}

		res, err = f.PostForm(host+string(api), form)
		if err != nil {
			return FunnelResponse{}, apiException.RequestError
		}
		if err = json.Unmarshal(res, &rc); err != nil {
			return FunnelResponse{}, apiException.RequestError
		}
		if rc.Code != 413 {
			break
		}
	}

	return rc, nil
}

// FetchHandleOfPost：
// - 非 ZF 接口：单节点调用（保留原逻辑）
// - ZF 接口：并发对冲到所有当前可用节点 + 简单熔断 / 恢复
func FetchHandleOfPost(form url.Values, host string, api funnelApi.FunnelApi) (interface{}, error) {
	loginType := funnelApi.LoginType(form.Get("type"))
	// 「是否 ZF 接口」用原来的约定：URL 中包含 "zf"
	zfFlag := strings.Contains(string(api), "zf")

	// 非 ZF 接口：保持原来的串行逻辑
	if !zfFlag {
		rc, err := singleHostRequest(context.Background(), host, api, form)
		if err != nil {
			// 原逻辑：对调用异常统一视为 ServerError
			return nil, apiException.ServerError
		}

		switch rc.Code {
		case 200:
			return rc.Data, nil
		case 413:
			return nil, apiException.ServerError
		case 412:
			return nil, apiException.NoThatPasswordOrWrong
		case 416:
			return nil, apiException.OAuthNotUpdate
		default:
			return nil, apiException.ServerError
		}
	}

	// 拿出当前健康的节点集合
	hosts := circuitBreaker.CB.LB.List(loginType)

	// 调用方通过 GetApi 传进来的 host 优先级最高，把它挪到列表最前面
	if host != "" {
		idx := -1
		for i, h := range hosts {
			if h == host {
				idx = i
				break
			}
		}

		if idx == -1 {
			// 原列表中没有这个 host，头插一份
			hosts = append([]string{host}, hosts...)
		} else if idx > 0 {
			// 已存在：挪到最前面，避免重复
			hosts = append([]string{host}, append(hosts[:idx], hosts[idx+1:]...)...)
		}
	}

	if len(hosts) == 0 {
		// 所有节点都已熔断
		return nil, apiException.NoApiAvailable
	}

	type result struct {
		host string
		rc   FunnelResponse
		err  error
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan result, len(hosts))
	var wg sync.WaitGroup

	// 并发对冲
	for _, h := range hosts {
		h := h
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 如果已经有其他节点成功了，尽量避免无意义请求
			select {
			case <-ctx.Done():
				return
			default:
			}

			rc, err := singleHostRequest(ctx, h, api, form)

			select {
			case resultCh <- result{host: h, rc: rc, err: err}:
			case <-ctx.Done():
				// 上层已经有结果了，丢弃即可
			}
		}()
	}

	// 等所有协程结束后关闭通道
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var firstErr error

	// 竞争结果：
	// - 第一个 200 直接返回数据，并标记该节点 Success
	// - 第一个 412 / 416 属于用户态错误，也直接返回，不再等其它节点
	// - 413 视为该节点过载/异常，Fail 一次，继续等待其它节点
	for r := range resultCh {
		if r.err != nil {
			// 网络 / 解析错误 / ctx 取消：认为节点异常
			circuitBreaker.CB.Fail(r.host, loginType)
			if firstErr == nil {
				// 统一向上映射成 ServerError
				firstErr = apiException.ServerError
			}
			continue
		}

		switch r.rc.Code {
		case 200:
			// 节点健康
			circuitBreaker.CB.Success(r.host, loginType)
			cancel()
			return r.rc.Data, nil

		case 412:
			// 密码错误：业务错误，节点本身是健康的
			circuitBreaker.CB.Success(r.host, loginType)
			cancel()
			return nil, apiException.NoThatPasswordOrWrong

		case 416:
			// OAuth 未更新：同样是业务错误，节点健康
			circuitBreaker.CB.Success(r.host, loginType)
			cancel()
			return nil, apiException.OAuthNotUpdate

		case 413:
			// 统一视为该节点过载，触发熔断统计，继续等其它节点
			circuitBreaker.CB.Fail(r.host, loginType)
			if firstErr == nil {
				firstErr = apiException.ServerError
			}

		default:
			// 其它错误码也视作节点异常
			circuitBreaker.CB.Fail(r.host, loginType)
			if firstErr == nil {
				firstErr = apiException.ServerError
			}
		}
	}

	if firstErr == nil {
		firstErr = apiException.ServerError
	}
	return nil, firstErr
}
