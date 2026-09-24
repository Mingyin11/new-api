package novelai

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type ChannelQueue struct {
	mu       sync.Mutex
	lastDone time.Time
}

var (
	channelQueues sync.Map
	rng           = rand.New(rand.NewSource(time.Now().UnixNano()))
	rngMu         sync.Mutex
)

func getChannelQueue(channelId int) *ChannelQueue {
	actual, _ := channelQueues.LoadOrStore(channelId, &ChannelQueue{})
	return actual.(*ChannelQueue)
}

// AcquireChannelLock 确保单个 NovelAI 渠道账号严格 1 并发，并在两次请求间注入 0.5s~3.0s 随机延迟
func AcquireChannelLock(ctx context.Context, channelId int) (func(), error) {
	q := getChannelQueue(channelId)
	q.mu.Lock()

	// 计算距离该渠道上一次生图完成经过的时间
	elapsed := time.Since(q.lastDone)

	rngMu.Lock()
	delayMs := 500 + rng.Intn(2501) // 500ms ~ 3000ms (0.5s - 3.0s)
	rngMu.Unlock()
	requiredDelay := time.Duration(delayMs) * time.Millisecond

	if !q.lastDone.IsZero() && elapsed < requiredDelay {
		sleepDuration := requiredDelay - elapsed
		select {
		case <-time.After(sleepDuration):
		case <-ctx.Done():
			q.mu.Unlock()
			return nil, ctx.Err()
		}
	}

	released := false
	releaseFunc := func() {
		if !released {
			released = true
			q.lastDone = time.Now()
			q.mu.Unlock()
		}
	}
	return releaseFunc, nil
}
