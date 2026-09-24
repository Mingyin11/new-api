package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// AdjustUserPower 原子的增减用户电量 (delta 为负表示扣减，允许变为负数)
func AdjustUserPower(userId int, delta int64) error {
	if userId <= 0 || delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ?", userId).Update("power", gorm.Expr("power + ?", delta))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// AdjustUserAnlas 原子的增减用户 Anlas (delta 为负表示扣减，允许变为负数)
func AdjustUserAnlas(userId int, delta int64) error {
	if userId <= 0 || delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ?", userId).Update("anlas", gorm.Expr("anlas + ?", delta))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// SetUserNovelAIBalance 直接设定用户的电量或 Anlas
func SetUserNovelAIBalance(userId int, power, anlas *int64) error {
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	updates := make(map[string]any)
	if power != nil {
		updates["power"] = *power
	}
	if anlas != nil {
		updates["anlas"] = *anlas
	}
	if len(updates) == 0 {
		return nil
	}
	result := DB.Model(&User{}).Where("id = ?", userId).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetUserNovelAIAllocation 配置用户的自动分配设置
func SetUserNovelAIAllocation(userId int, autoPower, autoAnlas bool, powerAmount, anlasAmount int) error {
	if userId <= 0 {
		return errors.New("invalid user id")
	}
	updates := map[string]any{
		"auto_power_enabled": autoPower,
		"auto_power_amount":  powerAmount,
		"auto_anlas_enabled": autoAnlas,
		"auto_anlas_amount":  anlasAmount,
	}
	result := DB.Model(&User{}).Where("id = ?", userId).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// BatchRefreshDailyPower 批量刷新全员今日电量 (针对 auto_power_enabled = true 且今日未刷新的用户)
func BatchRefreshDailyPower(today string) (int64, error) {
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&User{}).
			Where("auto_power_enabled = ? AND (last_power_refresh_date != ? OR last_power_refresh_date IS NULL OR last_power_refresh_date = '')", true, today).
			Updates(map[string]any{
				"power":                   gorm.Expr("auto_power_amount"),
				"last_power_refresh_date": today,
			})
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		return nil
	})
	if err != nil {
		common.SysError(fmt.Sprintf("BatchRefreshDailyPower failed: %v", err))
		return 0, err
	}
	return affected, nil
}

// BatchRefreshMonthlyAnlas 批量分配全员月度 Anlas (针对 auto_anlas_enabled = true 且当月未刷新的用户)
func BatchRefreshMonthlyAnlas(thisMonth string) (int64, error) {
	if thisMonth == "" {
		thisMonth = time.Now().Format("2006-01")
	}
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 累加月度 Anlas 配额
		result := tx.Model(&User{}).
			Where("auto_anlas_enabled = ? AND (last_anlas_refresh_month != ? OR last_anlas_refresh_month IS NULL OR last_anlas_refresh_month = '')", true, thisMonth).
			Updates(map[string]any{
				"anlas":                    gorm.Expr("anlas + auto_anlas_amount"),
				"last_anlas_refresh_month": thisMonth,
			})
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		return nil
	})
	if err != nil {
		common.SysError(fmt.Sprintf("BatchRefreshMonthlyAnlas failed: %v", err))
		return 0, err
	}
	return affected, nil
}

// CheckAndRefreshUserNovelAI 在用户请求时惰性补刷新其电量与 Anlas
func CheckAndRefreshUserNovelAI(user *User, today, thisMonth string) (bool, error) {
	if user == nil || user.Id <= 0 {
		return false, nil
	}
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	if thisMonth == "" {
		thisMonth = time.Now().Format("2006-01")
	}

	needPowerRefresh := user.AutoPowerEnabled && user.LastPowerRefreshDate != today
	needAnlasRefresh := user.AutoAnlasEnabled && user.LastAnlasRefreshMonth != thisMonth

	if !needPowerRefresh && !needAnlasRefresh {
		return false, nil
	}

	updates := make(map[string]any)
	if needPowerRefresh {
		updates["power"] = user.AutoPowerAmount
		updates["last_power_refresh_date"] = today
		user.Power = int64(user.AutoPowerAmount)
		user.LastPowerRefreshDate = today
	}
	if needAnlasRefresh {
		updates["anlas"] = gorm.Expr("anlas + ?", user.AutoAnlasAmount)
		updates["last_anlas_refresh_month"] = thisMonth
		user.Anlas += int64(user.AutoAnlasAmount)
		user.LastAnlasRefreshMonth = thisMonth
	}

	err := DB.Model(&User{}).Where("id = ?", user.Id).Updates(updates).Error
	if err != nil {
		common.SysError(fmt.Sprintf("CheckAndRefreshUserNovelAI for user %d failed: %v", user.Id, err))
		return false, err
	}
	return true, nil
}
