package controller

import (
	"net/http"
	"sync"

	coreController "github.com/CakeForKit/rsoi-lab2/lb-core/controller"
	"github.com/CakeForKit/rsoi-lab2/lb-core/model"
	"github.com/CakeForKit/rsoi-lab2/lb-core/utils"
	"github.com/CakeForKit/rsoi-lab2/ms-ticket/internal/service"
	"github.com/gin-gonic/gin"
)

var ticketCntrlr coreController.HttpController
var ticketCntrlrMutex sync.Mutex

type ticketController struct{ service service.TicketService }

func GetTicketController() (coreController.HttpController, error) {
	ticketCntrlrMutex.Lock()
	defer ticketCntrlrMutex.Unlock()
	if ticketCntrlr != nil {
		return ticketCntrlr, nil
	}
	srv, err := service.GetTicketService()
	if err != nil {
		return nil, err
	}
	ticketCntrlr = &ticketController{service: srv}
	return ticketCntrlr, nil
}
func (c *ticketController) RegisterHttpController(router *gin.Engine) {
	router.GET("/internal/tickets", c.list)
	router.GET("/internal/tickets/:ticketUid", c.get)
	router.POST("/internal/tickets", c.create)
	router.PUT("/internal/tickets/:ticketUid/cancel", c.cancel)
}
func username(ctx *gin.Context) string { return ctx.GetHeader("X-User-Name") }
func (c *ticketController) list(ctx *gin.Context) {
	result, err := c.service.GetAll(ctx, username(ctx))
	if err != nil {
		utils.AbortContextWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}
func (c *ticketController) get(ctx *gin.Context) {
	result, err := c.service.Get(ctx, username(ctx), ctx.Param("ticketUid"))
	if err != nil {
		utils.AbortContextWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}
func (c *ticketController) create(ctx *gin.Context) {
	var request model.TicketCreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || username(ctx) == "" || request.FlightNumber == "" || request.Price <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "invalid ticket"})
		return
	}
	result, err := c.service.Create(ctx, username(ctx), request)
	if err != nil {
		utils.AbortContextWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, result)
}
func (c *ticketController) cancel(ctx *gin.Context) {
	result, err := c.service.Cancel(ctx, username(ctx), ctx.Param("ticketUid"))
	if err != nil {
		utils.AbortContextWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}
