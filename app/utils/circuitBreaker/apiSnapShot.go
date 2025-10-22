package circuitBreaker

import (
	"time"

	cbConfig "wejh-go/config/circuitBreaker"
)

// ApiSnapShot 记录单 host+loginType 的访问状态
type ApiSnapShot struct {
	State      State
	ErrCount   Counter
	TotalCount Counter
	AccessLast time.Time
}

func NewApiSnapShot() *ApiSnapShot {
	return &ApiSnapShot{
		State:      Closed,
		ErrCount:   &atomicCounter{},
		TotalCount: &atomicCounter{},
		AccessLast: time.Time{},
	}
}

// Fail 返回 true 表示应当"打开熔断"
func (a *ApiSnapShot) Fail() bool {
	cfg := cbConfig.GetThresholdConfig()

	a.ErrCount.Add(1)
	a.TotalCount.Add(1)

	// 1) 连续失败直开
	if a.ErrCount.Get() >= int64(cfg.ConsecutiveFail) && a.State == Closed {
		a.State = Open
		return true
	}

	// 2) 错误率 + 最小样本量
	total := a.TotalCount.Get()
	if total >= int64(cfg.MinSample) && a.State == Closed {
		errRate := float64(a.ErrCount.Get()) / float64(total)
		if errRate >= cfg.ErrorRateOpen {
			a.State = Open
			return true
		}
	}
	return false
}

func (a *ApiSnapShot) Success() {
	a.TotalCount.Add(1)
	// 成功则认为从头开始累计失败（防止偶发错误放大）
	a.ErrCount.Zero()
	a.State = Closed
	a.AccessLast = time.Now()
}

type State int32

func (s State) String() string {
	switch s {
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALF_OPEN"
	case Closed:
		return "CLOSED"
	}
	return "INVALID"
}

const (
	Open State = iota
	HalfOpen
	Closed
)
