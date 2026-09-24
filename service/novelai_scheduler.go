package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

var (
	lastExecutedDate  string
	lastExecutedMonth string
	schedulerMu       sync.Mutex
)

// TriggerDailyPowerRefresh 管理员或系统手动触发全员每日电量刷新
func TriggerDailyPowerRefresh() (int64, error) {
	today := time.Now().Format("2006-01-02")
	affected, err := model.BatchRefreshDailyPower(today)
	if err != nil {
		return 0, err
	}
	schedulerMu.Lock()
	lastExecutedDate = today
	schedulerMu.Unlock()
	common.SysLog(fmt.Sprintf("[NovelAI Scheduler] 全员每日电量刷新成功，受影响用户数: %d (日期: %s)", affected, today))
	return affected, nil
}

// TriggerMonthlyAnlasRefresh 管理员或系统手动触发全员月度 Anlas 分配
func TriggerMonthlyAnlasRefresh() (int64, error) {
	thisMonth := time.Now().Format("2006-01")
	affected, err := model.BatchRefreshMonthlyAnlas(thisMonth)
	if err != nil {
		return 0, err
	}
	schedulerMu.Lock()
	lastExecutedMonth = thisMonth
	schedulerMu.Unlock()
	common.SysLog(fmt.Sprintf("[NovelAI Scheduler] 全员月度 Anlas 分配成功，受影响用户数: %d (月份: %s)", affected, thisMonth))
	return affected, nil
}

// InitNovelAIScheduler 启动 NovelAI 后台定时自驱刷新调度器
func InitNovelAIScheduler() {
	if !common.IsMasterNode {
		return
	}

	gopool.Go(func() {
		common.SysLog("[NovelAI Scheduler] 后台定时自动刷新调度器已启动")
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			today := now.Format("2006-01-02")
			thisMonth := now.Format("2006-01")

			schedulerMu.Lock()
			needDailyRefresh := lastExecutedDate != today
			needMonthlyRefresh := lastExecutedMonth != thisMonth
			schedulerMu.Unlock()

			// 检查每日电量刷新
			if needDailyRefresh {
				affected, err := model.BatchRefreshDailyPower(today)
				if err == nil {
					schedulerMu.Lock()
					lastExecutedDate = today
					schedulerMu.Unlock()
					if affected > 0 {
						common.SysLog(fmt.Sprintf("[NovelAI Scheduler] 每日电量自动刷新完成，刷新用户数: %d", affected))
					}
				}
			}

			// 检查月度 Anlas 分配
			if needMonthlyRefresh {
				affected, err := model.BatchRefreshMonthlyAnlas(thisMonth)
				if err == nil {
					schedulerMu.Lock()
					lastExecutedMonth = thisMonth
					schedulerMu.Unlock()
					if affected > 0 {
						common.SysLog(fmt.Sprintf("[NovelAI Scheduler] 月度 Anlas 自动分配完成，分配用户数: %d", affected))
					}
				}
			}
		}
	})
}
