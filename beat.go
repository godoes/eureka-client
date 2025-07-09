package eureka_client

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

type BeatReactor struct {
	config                   *Config
	beatMap                  ConcurrentMap
	clientBeatIntervalInSecs int64
	beatThreadCount          int
	beatThreadSemaphore      *semaphore.Weighted
	beatRecordMap            ConcurrentMap
	mux                      *sync.Mutex
	log                      Logger
	Period                   time.Duration
}

const DefaultBeatThreadNum = 20

var ctx = context.Background()

func NewBeatReactor(config *Config, clientBeatIntervalInSecs int64) BeatReactor {
	br := BeatReactor{
		config: config,
	}
	if clientBeatIntervalInSecs <= 0 {
		clientBeatIntervalInSecs = 5
	}
	br.beatMap = NewConcurrentMap()
	br.clientBeatIntervalInSecs = clientBeatIntervalInSecs
	br.beatThreadCount = DefaultBeatThreadNum
	br.beatRecordMap = NewConcurrentMap()
	br.beatThreadSemaphore = semaphore.NewWeighted(int64(br.beatThreadCount))
	br.mux = new(sync.Mutex)
	br.log = NewLogger(config.LogLevel)
	br.Period = time.Duration(clientBeatIntervalInSecs) * time.Second
	return br
}

func (br *BeatReactor) AddBeatInfo(beatInfo *Instance) {
	k := beatInfo.InstanceID
	defer br.mux.Unlock()
	br.mux.Lock()
	if data, ok := br.beatMap.Get(k); ok {
		beatInfo = data.(*Instance)
		beatInfo.Status = "UP"
		br.beatMap.Remove(k)
	}
	br.beatMap.Set(k, beatInfo)
	go br.sendInstanceBeat(k, beatInfo)
}

func (br *BeatReactor) RemoveBeatInfo(serviceName string, instanceId string) {
	log.Printf("remove beat: %s@%s from beat map", serviceName, instanceId)
	k := instanceId
	defer br.mux.Unlock()
	br.mux.Lock()
	data, exist := br.beatMap.Get(k)
	if exist {
		beatInfo := data.(*Instance)
		beatInfo.Status = "UP"
	}
	br.beatMap.Remove(k)
}

func (br *BeatReactor) sendInstanceBeat(k string, beatInfo *Instance) {
	for {
		err := br.beatThreadSemaphore.Acquire(ctx, 1)
		if err != nil {
			log.Printf("sendInstanceBeat failed to acquire semaphore: %v", err)
			return
		}
		//如果当前实例注销，则进行停止心跳
		if beatInfo.Status != "UP" {
			log.Printf("instance[%s] stop heartBeating", k)
			br.beatThreadSemaphore.Release(1)
			return
		}

		//进行心跳通信
		// /eureka/apps/ORDER-SERVICE/localhost:order-service:8886?status=UP
		err = Heartbeat(br.config.DefaultZone, beatInfo.App, beatInfo.InstanceID)
		//u := br.config.DefaultZone + "apps/" + beatInfo.App + "/" + beatInfo.InstanceID + "?status=UP"
		//result := requests.Put(u).Send().Status2xx()

		if err != nil {
			log.Printf("beat to server return error:%+v", err)
			if errors.Is(err, ErrNotFound) {
				log.Printf("can't find this instance, heart beat exist. key:%s", k)
				br.beatMap.Remove(k)
				return
			} else {
				br.beatThreadSemaphore.Release(1)
				t := time.NewTimer(br.Period)
				<-t.C
				continue
			}
		}

		br.beatRecordMap.Set(k, time.Now().UnixNano()/1e6)
		br.beatThreadSemaphore.Release(1)

		t := time.NewTimer(br.Period)
		<-t.C
	}
}
