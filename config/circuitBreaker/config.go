package circuitBreaker

import (
	"time"

	"wejh-go/config/config"

	"go.uber.org/zap"
)

// ---- LiveNess Probe ----

type LiveNessProbeConfig struct {
	StudentId     string
	OauthPassword string
	ZFPassword    string
	Duration      time.Duration
}

func GetLiveNessConfig() LiveNessProbeConfig {
	return LiveNessProbeConfig{
		StudentId:     config.Config.GetString("zfCircuit.studentId"),
		OauthPassword: config.Config.GetString("zfCircuit.oauthPassword"),
		ZFPassword:    config.Config.GetString("zfCircuit.zfPassword"),
		Duration:      config.Config.GetDuration("zfCircuit.duration"),
	}
}

// ---- LB 权重与对冲配置 ----

type WeightPerType struct {
	Oauth int `mapstructure:"oauth" json:"oauth" yaml:"oauth"`
	ZF    int `mapstructure:"zf"    json:"zf"    yaml:"zf"`
}

type Window struct {
	Name      string                   `mapstructure:"name"      json:"name"      yaml:"name"`
	Time      string                   `mapstructure:"time"      json:"time"      yaml:"time"` // 例如 "00:30-06:00"
	Overrides map[string]WeightPerType `mapstructure:"overrides" json:"overrides" yaml:"overrides"`
}

type HedgeConfig struct {
	Enable       bool          `mapstructure:"enable"       json:"enable"       yaml:"enable"`
	Delay        time.Duration `mapstructure:"delay"        json:"delay"        yaml:"delay"`
	MaxParallel  int           `mapstructure:"maxParallel"  json:"maxParallel"  yaml:"maxParallel"`
	OnlySameType bool          `mapstructure:"onlySameType" json:"onlySameType" yaml:"onlySameType"`
}

type RedisKeyConfig struct {
	WeightPrefix  string        `mapstructure:"weightPrefix"  json:"weightPrefix"  yaml:"weightPrefix"`
	CircuitPrefix string        `mapstructure:"circuitPrefix" json:"circuitPrefix" yaml:"circuitPrefix"`
	CacheTTL      time.Duration `mapstructure:"cacheTTL"      json:"cacheTTL"      yaml:"cacheTTL"`
}

type ThresholdConfig struct {
	MinSample       int     `mapstructure:"minSample"       json:"minSample"       yaml:"minSample"`
	ErrorRateOpen   float64 `mapstructure:"errorRateOpen"   json:"errorRateOpen"   yaml:"errorRateOpen"`
	ConsecutiveFail int     `mapstructure:"consecutiveFail" json:"consecutiveFail" yaml:"consecutiveFail"`
}

type LoadBalanceConfig struct {
	Apis          []string                 `mapstructure:"apis"          json:"apis"          yaml:"apis"`
	DefaultWeight int                      `mapstructure:"defaultWeight" json:"defaultWeight" yaml:"defaultWeight"`
	Weights       map[string]WeightPerType `mapstructure:"weights"       json:"weights"       yaml:"weights"`
	Windows       []Window                 `mapstructure:"windows"       json:"windows"       yaml:"windows"`
	Hedge         HedgeConfig              `mapstructure:"hedge"         json:"hedge"         yaml:"hedge"`
	Redis         RedisKeyConfig           `mapstructure:"redis"         json:"redis"         yaml:"redis"`
}

func GetLoadBalanceConfig() LoadBalanceConfig {
	cfg := LoadBalanceConfig{
		DefaultWeight: 100,
		Weights:       map[string]WeightPerType{},
		Windows:       []Window{},
		Hedge: HedgeConfig{
			Enable:       true,
			Delay:        200 * time.Millisecond,
			MaxParallel:  2,
			OnlySameType: true,
		},
		Redis: RedisKeyConfig{
			WeightPrefix:  "lb:weight",
			CircuitPrefix: "cb:open",
			CacheTTL:      30 * time.Second,
		},
	}
	// 顶层 apis（与原配置兼容）
	cfg.Apis = config.Config.GetStringSlice("zfCircuit.apis")

	// 读取 lb.* 分组
	if sub := config.Config.Sub("zfCircuit.lb"); sub != nil {
		_ = sub.Unmarshal(&cfg)
	}

	// 无论是否配置子项，Redis/Hedge/Windows/Weights 都保持 default，不必强依赖
	return cfg
}

func GetThresholdConfig() ThresholdConfig {
	th := ThresholdConfig{
		MinSample:       10,
		ErrorRateOpen:   0.5,
		ConsecutiveFail: 15,
	}
	if sub := config.Config.Sub("zfCircuit.threshold"); sub != nil {
		if err := sub.Unmarshal(&th); err != nil {
			zap.L().Warn("unmarshal threshold config failed, use default", zap.Error(err))
		}
	}
	return th
}
