package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type OAuthSleeperHandler struct {
	svc *service.OAuthSleeperService
}

func NewOAuthSleeperHandler(svc *service.OAuthSleeperService) *OAuthSleeperHandler {
	return &OAuthSleeperHandler{svc: svc}
}

func (h *OAuthSleeperHandler) GetStatus(c *gin.Context) {
	status, err := h.svc.GetStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *OAuthSleeperHandler) GetSettings(c *gin.Context) {
	settings, err := h.svc.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *OAuthSleeperHandler) UpdateSettings(c *gin.Context) {
	var req service.OAuthSleeperSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	settings, err := h.svc.SetSettings(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *OAuthSleeperHandler) ScanOnce(c *gin.Context) {
	result, err := h.svc.ScanOnce(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *OAuthSleeperHandler) ListEvents(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	events, pageResult, err := h.svc.ListEvents(c.Request.Context(), pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    "created_at",
		SortOrder: pagination.SortOrderDesc,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, events, &response.PaginationResult{
		Total:    pageResult.Total,
		Page:     pageResult.Page,
		PageSize: pageResult.PageSize,
		Pages:    pageResult.Pages,
	})
}
