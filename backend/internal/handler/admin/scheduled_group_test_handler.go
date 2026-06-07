package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ScheduledGroupTestHandler handles admin group scheduled-test-plan management.
type ScheduledGroupTestHandler struct {
	svc *service.ScheduledGroupTestService
}

func NewScheduledGroupTestHandler(svc *service.ScheduledGroupTestService) *ScheduledGroupTestHandler {
	return &ScheduledGroupTestHandler{svc: svc}
}

type createScheduledGroupTestPlanRequest struct {
	GroupID           int64  `json:"group_id" binding:"required"`
	AccountNameFilter string `json:"account_name_filter"`
	Enabled           *bool  `json:"enabled"`
}

type updateScheduledGroupTestPlanRequest struct {
	GroupID           int64   `json:"group_id"`
	AccountNameFilter *string `json:"account_name_filter"`
	Enabled           *bool   `json:"enabled"`
}

func (h *ScheduledGroupTestHandler) List(c *gin.Context) {
	plans, err := h.svc.ListPlans(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

func (h *ScheduledGroupTestHandler) Create(c *gin.Context) {
	var req createScheduledGroupTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.GroupID <= 0 {
		response.BadRequest(c, "group_id is required")
		return
	}

	plan := &service.ScheduledGroupTestPlan{
		GroupID:           req.GroupID,
		AccountNameFilter: req.AccountNameFilter,
		Enabled:           true,
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}

	created, err := h.svc.CreatePlan(c.Request.Context(), plan)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

func (h *ScheduledGroupTestHandler) Update(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	existing, err := h.svc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}

	var req updateScheduledGroupTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.GroupID > 0 {
		existing.GroupID = req.GroupID
	}
	if req.AccountNameFilter != nil {
		existing.AccountNameFilter = *req.AccountNameFilter
	}
	wasEnabled := existing.Enabled
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	updated, err := h.svc.UpdatePlan(c.Request.Context(), existing, wasEnabled)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *ScheduledGroupTestHandler) Delete(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	if err := h.svc.DeletePlan(c.Request.Context(), planID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
