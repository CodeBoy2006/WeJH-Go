package funnelServices

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"wejh-go/app/apiException"
	"wejh-go/app/utils/circuitBreaker"
	"wejh-go/app/utils/fetch"
	"wejh-go/config/api/funnelApi"
)

// FunnelResponse 对 funnel 的统一响应
type FunnelResponse struct {
	Code int         `json:"code" binding:"required"`
	Msg  string      `json:"message" binding:"required"`
	Data interface{} `json:"data"`
}

// FetchHandleOfPost 原有：单路请求（兼容库内其它模块）
func FetchHandleOfPost(form url.Values, host string, url funnelApi.FunnelApi) (interface{}, error) {
	f := fetch.Fetch{}
	f.Init()
	var rc FunnelResponse
	var res []byte
	var err error
	for i := 0; i < 5; i++ {
		res, err = f.PostForm(host+string(url), form)
		if err != nil {
			err = apiException.RequestError
			break
		}
		if err = json.Unmarshal(res, &rc); err != nil {
			err = apiException.RequestError
			break
		}
		if rc.Code != 413 {
			break
		}
	}

	loginType := funnelApi.LoginType(form.Get("type"))
	zfFlag := strings.Contains(string(url), "zf")

	if err != nil {
		if zfFlag {
			circuitBreaker.CB.Fail(host, loginType)
		}
		return nil, apiException.ServerError
	}
	if zfFlag {
		if rc.Code == 200 || rc.Code == 412 || rc.Code == 416 {
			circuitBreaker.CB.Success(host, loginType)
		} else {
			circuitBreaker.CB.Fail(host, loginType)
		}
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

// FetchHandleOfPostHedged 对冲版：hosts 至少 1 个；可延迟发起第 2 路；返回 (data, usedHost)
func FetchHandleOfPostHedged(form url.Values, hosts []string, url funnelApi.FunnelApi, hedgeDelay time.Duration) (interface{}, string, error) {
	if len(hosts) == 0 {
		return nil, "", apiException.ServerError
	}
	if len(hosts) > 2 {
		hosts = hosts[:2]
	}

	type result struct {
		data interface{}
		host string
		err  error
	}

	loginType := funnelApi.LoginType(form.Get("type"))
	zfFlag := strings.Contains(string(url), "zf")

	doReq := func(host string, out chan<- result) {
		start := time.Now()
		f := fetch.Fetch{}
		f.Init()
		var rc FunnelResponse
		var res []byte
		var err error
		for i := 0; i < 5; i++ {
			res, err = f.PostForm(host+string(url), form)
			if err != nil {
				err = apiException.RequestError
				break
			}
			if err = json.Unmarshal(res, &rc); err != nil {
				err = apiException.RequestError
				break
			}
			if rc.Code != 413 {
				break
			}
		}

		cost := time.Since(start)
		zap.L().Info("funnel request done",
			zap.String("host", host),
			zap.String("loginType", string(loginType)),
			zap.String("api", string(url)),
			zap.Duration("cost", cost),
			zap.Int("code", rc.Code),
			zap.Bool("hedged", true),
		)

		if err != nil {
			if zfFlag {
				circuitBreaker.CB.Fail(host, loginType)
			}
			out <- result{nil, host, apiException.ServerError}
			return
		}
		if zfFlag {
			if rc.Code == 200 || rc.Code == 412 || rc.Code == 416 {
				circuitBreaker.CB.Success(host, loginType)
			} else {
				circuitBreaker.CB.Fail(host, loginType)
			}
		}
		switch rc.Code {
		case 200:
			out <- result{rc.Data, host, nil}
		case 413:
			out <- result{nil, host, apiException.ServerError}
		case 412:
			out <- result{nil, host, apiException.NoThatPasswordOrWrong}
		case 416:
			out <- result{nil, host, apiException.OAuthNotUpdate}
		default:
			out <- result{nil, host, apiException.ServerError}
		}
	}

	ch := make(chan result, len(hosts))
	// 第一条立即发起
	go doReq(hosts[0], ch)

	// 若存在第 2 条，并配置了延迟，则延迟后发起
	if len(hosts) == 2 {
		if hedgeDelay <= 0 {
			go doReq(hosts[1], ch)
		} else {
			go func() {
				time.Sleep(hedgeDelay)
				go doReq(hosts[1], ch)
			}()
		}
	}

	// 取第一个成功/可判定的结果
	first := <-ch
	if first.err == nil {
		return first.data, first.host, nil
	}
	// 如果只有 1 条，直接返回；若有 2 条，取第二个
	if len(hosts) == 1 {
		return nil, first.host, first.err
	}
	second := <-ch
	return second.data, second.host, second.err
}
