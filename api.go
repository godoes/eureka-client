package eureka_client

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/godoes/eureka-client/requests"
)

var (
	// ErrNotFound 实例不存在，需要重新注册
	ErrNotFound = errors.New("not found")
)

// 与 eureka 服务端 rest 交互
// https://github.com/Netflix/eureka/wiki/Eureka-REST-operations

// Register 注册实例
// POST /eureka/v2/apps/appID
func Register(zone, app string, instance *Instance) error {
	// Instance 服务实例
	type InstanceInfo struct {
		Instance *Instance `json:"instance"`
	}
	info := &InstanceInfo{
		Instance: instance,
	}

	urlPath := fmt.Sprintf("apps/%s", url.PathEscape(app))
	u, err := buildURL(zone, urlPath)
	if err != nil {
		return err
	}

	result := requests.Post(u).Header("Accept", "application/json").Json(info).Send().Status2xx()
	if result.Err == nil {
		instance.Beater.AddBeatInfo(instance)
	}
	return result.Err
}

// UnRegister 删除实例
// DELETE /eureka/v2/apps/appID/instanceID
func UnRegister(zone, app string, instance *Instance) error {
	urlPath := fmt.Sprintf("apps/%s/%s", url.PathEscape(app), url.PathEscape(instance.InstanceID))
	u, err := buildURL(zone, urlPath)
	if err != nil {
		return err
	}

	result := requests.Delete(u).Header("Accept", "application/json").Send().StatusOk()
	if result.Err == nil && instance.Beater != nil {
		instance.Beater.RemoveBeatInfo(app, instance.InstanceID)
	}
	return result.Err
}

// Refresh 查询所有服务实例
// GET /eureka/v2/apps
func Refresh(zone string) (*Applications, error) {
	type Result struct {
		Applications *Applications `json:"applications"`
	}
	apps := new(Applications)
	res := &Result{
		Applications: apps,
	}

	urlPath := "apps"
	u, err := buildURL(zone, urlPath)
	if err != nil {
		return nil, err
	}

	err = requests.Get(u).Header("Accept", "application/json").Send().StatusOk().Json(res)
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// Heartbeat 发送心跳
// PUT /eureka/v2/apps/appID/instanceID
func Heartbeat(zone, app, instanceID string) error {
	urlPath := fmt.Sprintf("apps/%s/%s", url.PathEscape(app), url.PathEscape(instanceID))
	u, err := buildURL(zone, urlPath)
	if err != nil {
		return err
	}

	params := url.Values{
		"status": {"UP"},
	}
	result := requests.Put(u).Params(params).Header("Accept", "application/json").Send()
	if result.Err != nil {
		return result.Err
	}

	switch result.Resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		// 心跳 404 说明eureka server重启过，需要重新注册
		return ErrNotFound
	default:
		return fmt.Errorf("heartbeat failed, invalid status code: %d", result.Resp.StatusCode)
	}
}

// 构建完整的 Eureka 请求地址
func buildURL(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid zone URL: %w", err)
	}
	if u.Path, err = url.JoinPath(u.Path, path); err != nil {
		return "", fmt.Errorf("failed to join path: %s", path)
	}
	return u.String(), nil
}
