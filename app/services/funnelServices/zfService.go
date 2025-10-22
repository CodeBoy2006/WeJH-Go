package funnelServices

import (
	"net/url"
	"time"

	"wejh-go/app/models"
	"wejh-go/config/api/funnelApi"
	cbConfig "wejh-go/config/circuitBreaker"
)

func genTermForm(u *models.User, year, term string, loginType funnelApi.LoginType) url.Values {
	var password string

	if loginType == funnelApi.Oauth {
		password = u.OauthPassword
	} else {
		password = u.ZFPassword
	}

	form := url.Values{}
	form.Add("username", u.StudentID)
	form.Add("password", password)
	form.Add("type", string(loginType))
	form.Add("year", year)
	form.Add("term", term)
	return form
}

// ---------- 单路版（保留兼容） ----------

func GetClassTable(u *models.User, year, term, host string, loginType funnelApi.LoginType) (interface{}, error) {
	form := genTermForm(u, year, term, loginType)
	return FetchHandleOfPost(form, host, funnelApi.ZFClassTable)
}

func GetScore(u *models.User, year, term, host string, loginType funnelApi.LoginType) (interface{}, error) {
	form := genTermForm(u, year, term, loginType)
	return FetchHandleOfPost(form, host, funnelApi.ZFScore)
}

func GetMidTermScore(u *models.User, year, term, host string, loginType funnelApi.LoginType) (interface{}, error) {
	form := genTermForm(u, year, term, loginType)
	return FetchHandleOfPost(form, host, funnelApi.ZFMidTermScore)
}

func GetExam(u *models.User, year, term, host string, loginType funnelApi.LoginType) (interface{}, error) {
	form := genTermForm(u, year, term, loginType)
	return FetchHandleOfPost(form, host, funnelApi.ZFExam)
}

func GetRoom(u *models.User, year, term, campus, weekday, week, sections, host string, loginType funnelApi.LoginType) (interface{}, error) {
	form := genTermForm(u, year, term, loginType)
	form.Add("campus", campus)
	form.Add("weekday", weekday)
	form.Add("week", week)
	form.Add("sections", sections)
	return FetchHandleOfPost(form, host, funnelApi.ZFRoom)
}

func BindPassword(u *models.User, year, term, host string, loginType funnelApi.LoginType) (interface{}, error) {
	var password string
	if loginType == "ZF" {
		password = u.ZFPassword
	} else if loginType == "OAUTH" {
		password = u.OauthPassword
	}
	form := url.Values{}
	form.Add("username", u.StudentID)
	form.Add("password", password)
	form.Add("type", string(loginType))
	form.Add("year", year)
	form.Add("term", term)
	return FetchHandleOfPost(form, host, funnelApi.ZFExam)
}

// ---------- Hedged 版 ----------

func GetClassTableHedged(u *models.User, year, term string, hosts []string, loginType funnelApi.LoginType) (interface{}, string, error) {
	form := genTermForm(u, year, term, loginType)
	hedgeDelay := cbConfig.GetLoadBalanceConfig().Hedge.Delay
	return FetchHandleOfPostHedged(form, hosts, funnelApi.ZFClassTable, hedgeDelay)
}

func GetScoreHedged(u *models.User, year, term string, hosts []string, loginType funnelApi.LoginType) (interface{}, string, error) {
	form := genTermForm(u, year, term, loginType)
	hedgeDelay := cbConfig.GetLoadBalanceConfig().Hedge.Delay
	return FetchHandleOfPostHedged(form, hosts, funnelApi.ZFScore, hedgeDelay)
}

func GetMidTermScoreHedged(u *models.User, year, term string, hosts []string, loginType funnelApi.LoginType) (interface{}, string, error) {
	form := genTermForm(u, year, term, loginType)
	hedgeDelay := cbConfig.GetLoadBalanceConfig().Hedge.Delay
	return FetchHandleOfPostHedged(form, hosts, funnelApi.ZFMidTermScore, hedgeDelay)
}

func GetExamHedged(u *models.User, year, term string, hosts []string, loginType funnelApi.LoginType) (interface{}, string, error) {
	form := genTermForm(u, year, term, loginType)
	hedgeDelay := cbConfig.GetLoadBalanceConfig().Hedge.Delay
	return FetchHandleOfPostHedged(form, hosts, funnelApi.ZFExam, hedgeDelay)
}

func GetRoomHedged(u *models.User, year, term, campus, weekday, week, sections string, hosts []string, loginType funnelApi.LoginType) (interface{}, string, error) {
	form := genTermForm(u, year, term, loginType)
	form.Add("campus", campus)
	form.Add("weekday", weekday)
	form.Add("week", week)
	form.Add("sections", sections)
	hedgeDelay := cbConfig.GetLoadBalanceConfig().Hedge.Delay
	return FetchHandleOfPostHedged(form, hosts, funnelApi.ZFRoom, hedgeDelay)
}
