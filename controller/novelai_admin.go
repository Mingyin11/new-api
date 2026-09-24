package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type NovelAIUserAdjustRequest struct {
	UserId           int    `json:"user_id"`
	Power            *int64 `json:"power"`
	Anlas            *int64 `json:"anlas"`
	AutoPowerEnabled *bool  `json:"auto_power_enabled"`
	AutoPowerAmount  *int   `json:"auto_power_amount"`
	AutoAnlasEnabled *bool  `json:"auto_anlas_enabled"`
	AutoAnlasAmount  *int   `json:"auto_anlas_amount"`
}

// AdjustUserNovelAI 管理员调整用户的电量/Anlas及自动分配策略
func AdjustUserNovelAI(c *gin.Context) {
	var req NovelAIUserAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request body: " + err.Error()})
		return
	}

	if req.UserId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user_id"})
		return
	}

	user, err := model.GetUserById(req.UserId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "User not found"})
		return
	}

	myRole := c.GetInt("role")
	if !canManageTargetRole(myRole, user.Role) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Permission denied"})
		return
	}

	// 1. 更新电量或 Anlas 余额
	if req.Power != nil || req.Anlas != nil {
		if err := model.SetUserNovelAIBalance(req.UserId, req.Power, req.Anlas); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update balance: " + err.Error()})
			return
		}
	}

	// 2. 更新自动分配策略
	autoPower := user.AutoPowerEnabled
	if req.AutoPowerEnabled != nil {
		autoPower = *req.AutoPowerEnabled
	}
	powerAmount := user.AutoPowerAmount
	if req.AutoPowerAmount != nil {
		powerAmount = *req.AutoPowerAmount
	}

	autoAnlas := user.AutoAnlasEnabled
	if req.AutoAnlasEnabled != nil {
		autoAnlas = *req.AutoAnlasEnabled
	}
	anlasAmount := user.AutoAnlasAmount
	if req.AutoAnlasAmount != nil {
		anlasAmount = *req.AutoAnlasAmount
	}

	if req.AutoPowerEnabled != nil || req.AutoPowerAmount != nil || req.AutoAnlasEnabled != nil || req.AutoAnlasAmount != nil {
		if err := model.SetUserNovelAIAllocation(req.UserId, autoPower, autoAnlas, powerAmount, anlasAmount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update allocation: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "NovelAI user settings updated successfully"})
}

// TriggerDailyPowerRefreshAdmin 管理员一键触发全员每日电量刷新
func TriggerDailyPowerRefreshAdmin(c *gin.Context) {
	affected, err := service.TriggerDailyPowerRefresh()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to refresh daily power: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Daily power refreshed successfully",
		"affected": affected,
	})
}

// TriggerMonthlyAnlasRefreshAdmin 管理员一键触发全员月度 Anlas 分配
func TriggerMonthlyAnlasRefreshAdmin(c *gin.Context) {
	affected, err := service.TriggerMonthlyAnlasRefresh()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to refresh monthly anlas: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Monthly anlas refreshed successfully",
		"affected": affected,
	})
}

// GetUserNovelAIInfo 获取单个用户的 NovelAI 详细状态
func GetUserNovelAIInfo(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		userId = c.GetInt("id")
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "User not found"})
		return
	}

	myRole := c.GetInt("role")
	myId := c.GetInt("id")
	if myRole < common.RoleAdminUser && myId != userId {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Permission denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":                  user.Id,
			"username":                 user.Username,
			"power":                    user.Power,
			"anlas":                    user.Anlas,
			"auto_power_enabled":       user.AutoPowerEnabled,
			"auto_power_amount":        user.AutoPowerAmount,
			"auto_anlas_enabled":       user.AutoAnlasEnabled,
			"auto_anlas_amount":        user.AutoAnlasAmount,
			"last_power_refresh_date":  user.LastPowerRefreshDate,
			"last_anlas_refresh_month": user.LastAnlasRefreshMonth,
		},
	})
}
