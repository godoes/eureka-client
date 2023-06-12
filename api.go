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
	var info = &InstanceInfo{
		Instance: instance,
	}

	u := zone + "apps/" + app

	// status: http.StatusNoContent
	result := requests.Post(u).Json(info).Send().Status2xx()
	if result.Err == nil {
		instance.Beater.AddBeatInfo(instance)
	}
	return result.Err
}

// UnRegister 删除实例
// DELETE /eureka/v2/apps/appID/instanceID
func UnRegister(zone, app string, instance *Instance) error {
	u := zone + "apps/" + app + "/" + instance.InstanceID
	// status: http.StatusNoContent
	result := requests.Delete(u).Send().StatusOk()
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
	u := zone + "apps"
	err := requests.Get(u).Header("Accept", " application/json").Send().StatusOk().Json(res)
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// Heartbeat 发送心跳
// PUT /eureka/v2/apps/appID/instanceID
func Heartbeat(zone, app, instanceID string) error {
	u := zone + "apps/" + app + "/" + instanceID
	params := url.Values{
		"status": {"UP"},
	}
	result := requests.Put(u).Params(params).Send()
	if result.Err != nil {
		return result.Err
	}
	// 心跳 404 说明eureka server重启过，需要重新注册
	if result.Resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if result.Resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed, invalid status code: %d", result.Resp.StatusCode)
	}
	return nil
}
