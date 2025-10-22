package circuitBreaker

import (
	"sync"

	"go.uber.org/zap"

	"wejh-go/config/api/funnelApi"
	cbConfig "wejh-go/config/circuitBreaker"
)

var CB CircuitBreaker

type CircuitBreaker struct {
	LB       LoadBalance
	SnapShot *sync.Map // key: host+loginType -> *ApiSnapShot
}

func Init() {
	// 读配置
	lbCfg := cbConfig.GetLoadBalanceConfig()
	Probe = NewLiveNessProbe(cbConfig.GetLiveNessConfig())

	// 初始化带权重 LB
	lb := NewLoadBalance(lbCfg)

	// 快照初始化
	snapShot := &sync.Map{}
	for _, api := range lbCfg.Apis {
		// 两种登录方式均注册
		lb.Add(api, funnelApi.Oauth)
		lb.Add(api, funnelApi.ZF)

		snapShot.Store(api+string(funnelApi.Oauth), NewApiSnapShot())
		snapShot.Store(api+string(funnelApi.ZF), NewApiSnapShot())
	}

	CB = CircuitBreaker{
		LB:       lb,
		SnapShot: snapShot,
	}
	zap.L().Info("circuit breaker initialized")
}

func (c *CircuitBreaker) GetApi(zfFlag, oauthFlag bool) (string, funnelApi.LoginType, error) {
	return c.LB.Pick(zfFlag, oauthFlag)
}

// PickN：用于对冲，同一登录方式返回多个不同 host
func (c *CircuitBreaker) PickN(zfFlag, oauthFlag bool, n int) ([]string, funnelApi.LoginType, error) {
	return c.LB.PickN(zfFlag, oauthFlag, n)
}

func (c *CircuitBreaker) Fail(api string, loginType funnelApi.LoginType) {
	value, _ := c.SnapShot.Load(api + string(loginType))
	s := value.(*ApiSnapShot)
	if s.Fail() {
		// 达到熔断条件
		c.LB.Remove(api, loginType)
		Probe.Add(api, loginType)
	}
}

func (c *CircuitBreaker) Success(api string, loginType funnelApi.LoginType) {
	value, _ := c.SnapShot.Load(api + string(loginType))
	value.(*ApiSnapShot).Success()
}
