package zfController

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"wejh-go/app/apiException"
	"wejh-go/app/services/funnelServices"
	"wejh-go/app/services/sessionServices"
	"wejh-go/app/services/userServices"
	"wejh-go/app/utils"
	"wejh-go/app/utils/circuitBreaker"
	cbConfig "wejh-go/config/circuitBreaker"
	"wejh-go/config/redis"
)

type form struct {
	Year string `json:"year" binding:"required"`
	Term string `json:"term" binding:"required"`
}

func GetClassTable(c *gin.Context) {
	var postForm form
	if err := c.ShouldBindJSON(&postForm); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	user, err := sessionServices.GetUserSession(c)
	if err != nil {
		apiException.AbortWithException(c, apiException.NotLogin, err)
		return
	}

	hedge := cbConfig.GetLoadBalanceConfig().Hedge
	if hedge.Enable {
		hosts, loginType, err := circuitBreaker.CB.PickN(user.ZFPassword != "", user.OauthPassword != "", hedge.MaxParallel)
		if err != nil {
			apiException.AbortWithError(c, err)
			return
		}
		ret, usedHost, err := funnelServices.GetClassTableHedged(user, postForm.Year, postForm.Term, hosts, loginType)
		if err != nil {
			userServices.DelPassword(err, user, string(loginType))
			apiException.AbortWithError(c, err)
			return
		}
		c.Header("X-Upstream-Api", usedHost)
		c.Header("X-Login-Type", string(loginType))
		utils.JsonSuccessResponse(c, ret)
		return
	}

	// 兼容：非 hedge
	api, loginType, err := circuitBreaker.CB.GetApi(user.ZFPassword != "", user.OauthPassword != "")
	if err != nil {
		apiException.AbortWithError(c, err)
		return
	}
	result, err := funnelServices.GetClassTable(user, postForm.Year, postForm.Term, api, loginType)
	if err != nil {
		userServices.DelPassword(err, user, string(loginType))
		apiException.AbortWithError(c, err)
		return
	}
	c.Header("X-Upstream-Api", api)
	c.Header("X-Login-Type", string(loginType))
	utils.JsonSuccessResponse(c, result)
}

func GetScore(c *gin.Context) {
	var postForm form
	if err := c.ShouldBindJSON(&postForm); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	user, err := sessionServices.GetUserSession(c)
	if err != nil {
		apiException.AbortWithException(c, apiException.NotLogin, err)
		return
	}

	hedge := cbConfig.GetLoadBalanceConfig().Hedge
	if hedge.Enable {
		hosts, loginType, err := circuitBreaker.CB.PickN(user.ZFPassword != "", user.OauthPassword != "", hedge.MaxParallel)
		if err != nil {
			apiException.AbortWithError(c, err)
			return
		}
		ret, usedHost, err := funnelServices.GetScoreHedged(user, postForm.Year, postForm.Term, hosts, loginType)
		if err != nil {
			userServices.DelPassword(err, user, string(loginType))
			apiException.AbortWithError(c, err)
			return
		}
		c.Header("X-Upstream-Api", usedHost)
		c.Header("X-Login-Type", string(loginType))
		utils.JsonSuccessResponse(c, ret)
		return
	}

	api, loginType, err := circuitBreaker.CB.GetApi(user.ZFPassword != "", user.OauthPassword != "")
	if err != nil {
		apiException.AbortWithError(c, err)
		return
	}
	result, err := funnelServices.GetScore(user, postForm.Year, postForm.Term, api, loginType)
	if err != nil {
		userServices.DelPassword(err, user, string(loginType))
		apiException.AbortWithError(c, err)
		return
	}
	c.Header("X-Upstream-Api", api)
	c.Header("X-Login-Type", string(loginType))
	utils.JsonSuccessResponse(c, result)
}

func GetMidTermScore(c *gin.Context) {
	var postForm form
	if err := c.ShouldBindJSON(&postForm); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	user, err := sessionServices.GetUserSession(c)
	if err != nil {
		apiException.AbortWithException(c, apiException.NotLogin, err)
		return
	}

	hedge := cbConfig.GetLoadBalanceConfig().Hedge
	if hedge.Enable {
		hosts, loginType, err := circuitBreaker.CB.PickN(user.ZFPassword != "", user.OauthPassword != "", hedge.MaxParallel)
		if err != nil {
			apiException.AbortWithError(c, err)
			return
		}
		ret, usedHost, err := funnelServices.GetMidTermScoreHedged(user, postForm.Year, postForm.Term, hosts, loginType)
		if err != nil {
			userServices.DelPassword(err, user, string(loginType))
			apiException.AbortWithError(c, err)
			return
		}
		c.Header("X-Upstream-Api", usedHost)
		c.Header("X-Login-Type", string(loginType))
		utils.JsonSuccessResponse(c, ret)
		return
	}

	api, loginType, err := circuitBreaker.CB.GetApi(user.ZFPassword != "", user.OauthPassword != "")
	if err != nil {
		apiException.AbortWithError(c, err)
		return
	}
	result, err := funnelServices.GetMidTermScore(user, postForm.Year, postForm.Term, api, loginType)
	if err != nil {
		userServices.DelPassword(err, user, string(loginType))
		apiException.AbortWithError(c, err)
		return
	}
	c.Header("X-Upstream-Api", api)
	c.Header("X-Login-Type", string(loginType))
	utils.JsonSuccessResponse(c, result)
}

func GetExam(c *gin.Context) {
	var postForm form
	if err := c.ShouldBindJSON(&postForm); err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	user, err := sessionServices.GetUserSession(c)
	if err != nil {
		apiException.AbortWithException(c, apiException.NotLogin, err)
		return
	}

	hedge := cbConfig.GetLoadBalanceConfig().Hedge
	if hedge.Enable {
		hosts, loginType, err := circuitBreaker.CB.PickN(user.ZFPassword != "", user.OauthPassword != "", hedge.MaxParallel)
		if err != nil {
			apiException.AbortWithError(c, err)
			return
		}
		ret, usedHost, err := funnelServices.GetExamHedged(user, postForm.Year, postForm.Term, hosts, loginType)
		if err != nil {
			userServices.DelPassword(err, user, string(loginType))
			apiException.AbortWithError(c, err)
			return
		}
		c.Header("X-Upstream-Api", usedHost)
		c.Header("X-Login-Type", string(loginType))
		utils.JsonSuccessResponse(c, ret)
		return
	}

	api, loginType, err := circuitBreaker.CB.GetApi(user.ZFPassword != "", user.OauthPassword != "")
	if err != nil {
		apiException.AbortWithError(c, err)
		return
	}
	result, err := funnelServices.GetExam(user, postForm.Year, postForm.Term, api, loginType)
	if err != nil {
		userServices.DelPassword(err, user, string(loginType))
		apiException.AbortWithError(c, err)
		return
	}
	c.Header("X-Upstream-Api", api)
	c.Header("X-Login-Type", string(loginType))
	utils.JsonSuccessResponse(c, result)
}

type roomForm struct {
	Year     string `json:"year" binding:"required"`
	Term     string `json:"term" binding:"required"`
	Campus   string `json:"campus" binding:"required"`
	Weekday  string `json:"weekday" binding:"required"`
	Sections string `json:"sections" binding:"required"`
	Week     string `json:"week" binding:"required"`
}

func GetRoom(c *gin.Context) {
	var postForm roomForm
	err := c.ShouldBindJSON(&postForm)
	if err != nil {
		apiException.AbortWithException(c, apiException.ParamError, err)
		return
	}
	user, err := sessionServices.GetUserSession(c)

	if err != nil {
		apiException.AbortWithException(c, apiException.NotLogin, err)
		return
	}

	// 使用 Redis 缓存键，包含查询参数
	cacheKey := fmt.Sprintf("room:%s:%s:%s:%s:%s:%s", postForm.Year, postForm.Term, postForm.Campus, postForm.Weekday, postForm.Week, postForm.Sections)

	// 从 Redis 中获取缓存结果
	cachedResult, cacheErr := redis.RedisClient.Get(c, cacheKey).Result()
	if cacheErr == nil {
		var result []map[string]interface{}
		if err := json.Unmarshal([]byte(cachedResult), &result); err == nil {
			utils.JsonSuccessResponse(c, result)
			return
		} else {
			apiException.AbortWithException(c, apiException.ServerError, err)
			return
		}
	}

	hedge := cbConfig.GetLoadBalanceConfig().Hedge
	var result interface{}
	var usedHost string
	var loginType string

	if hedge.Enable {
		hosts, lt, err := circuitBreaker.CB.PickN(user.ZFPassword != "", user.OauthPassword != "", hedge.MaxParallel)
		if err != nil {
			apiException.AbortWithError(c, err)
			return
		}
		result, usedHost, err = funnelServices.GetRoomHedged(user, postForm.Year, postForm.Term, postForm.Campus, postForm.Weekday, postForm.Week, postForm.Sections, hosts, lt)
		loginType = string(lt)
		if err != nil {
			userServices.DelPassword(err, user, loginType)
			apiException.AbortWithError(c, err)
			return
		}
	} else {
		api, lt, err := circuitBreaker.CB.GetApi(user.ZFPassword != "", user.OauthPassword != "")
		if err != nil {
			apiException.AbortWithError(c, err)
			return
		}
		result, err = funnelServices.GetRoom(user, postForm.Year, postForm.Term, postForm.Campus, postForm.Weekday, postForm.Week, postForm.Sections, api, lt)
		usedHost = api
		loginType = string(lt)
		if err != nil {
			userServices.DelPassword(err, user, loginType)
			apiException.AbortWithError(c, err)
			return
		}
	}

	// 将结果缓存到 Redis 中
	if result != nil {
		resultJson, _ := json.Marshal(result)
		err = redis.RedisClient.Set(c, cacheKey, string(resultJson), 1*time.Hour).Err()
		if err != nil {
			apiException.AbortWithException(c, apiException.ServerError, err)
			return
		}
	}

	c.Header("X-Upstream-Api", usedHost)
	c.Header("X-Login-Type", loginType)
	utils.JsonSuccessResponse(c, result)
}
