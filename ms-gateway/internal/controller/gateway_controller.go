package controller

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strconv"
	"sync"

	coreConroller "github.com/CakeForKit/rsoi-lab2/lb-core/controller"
	"github.com/CakeForKit/rsoi-lab2/lb-core/model"
	"github.com/CakeForKit/rsoi-lab2/ms-gateway/internal/proxy"
	"github.com/CakeForKit/rsoi-lab2/ms-gateway/internal/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var gatewayCntrlr coreConroller.HttpController
var gatewayCntrlrMutex sync.Mutex

type gatewayController struct {
	proxy   *httputil.ReverseProxy
	service *service.GatewayService
}

func GetGatewayController() (coreConroller.HttpController, error) {
	gatewayCntrlrMutex.Lock()
	defer gatewayCntrlrMutex.Unlock()

	if gatewayCntrlr != nil {
		return gatewayCntrlr, nil
	}
	gatewayCntrlr = &gatewayController{
		proxy:   proxy.NewGatewayProxy(),
		service: service.New(),
	}
	return gatewayCntrlr, nil
}

func (controller gatewayController) RegisterHttpController(router *gin.Engine) {
	api := router.Group("/api/v1")
	api.GET("/flights", controller.flights)
	api.GET("/privilege", controller.privilege)
	api.POST("/tickets", controller.purchase)
	api.GET("/tickets", controller.tickets)
	api.GET("/tickets/:ticketUid", controller.ticket)
	api.DELETE("/tickets/:ticketUid", controller.cancel)
	api.GET("/me", controller.me)
	router.NoRoute(controller.transfer)
}

func username(ctx *gin.Context) (string, bool) {
	value := ctx.GetHeader("X-User-Name")
	if value == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "X-User-Name is required"})
		return "", false
	}
	return value, true
}

func failure(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
}

func (controller gatewayController) flights(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("size", "10"))
	result, err := controller.service.Flights(page, size)
	if err != nil {
		failure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (controller gatewayController) privilege(ctx *gin.Context) {
	user, ok := username(ctx)
	if !ok {
		return
	}
	result, err := controller.service.GetBonus(user)
	if err != nil {
		failure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (controller gatewayController) purchase(ctx *gin.Context) {
	user, ok := username(ctx)
	if !ok {
		return
	}
	var request model.TicketPurchaseRequest
	if ctx.ShouldBindJSON(&request) != nil {
		failure(ctx, http.ErrNotSupported)
		return
	}
	result, err := controller.service.Purchase(user, request)
	if err != nil {
		failure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (controller gatewayController) tickets(ctx *gin.Context) {
	user, ok := username(ctx)
	if !ok {
		return
	}
	result, err := controller.service.Tickets(user)
	if err != nil {
		failure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (controller gatewayController) ticket(ctx *gin.Context) {
	user, ok := username(ctx)
	if !ok {
		return
	}
	result, err := controller.service.Ticket(user, ctx.Param("ticketUid"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "ticket not found"})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (controller gatewayController) cancel(ctx *gin.Context) {
	user, ok := username(ctx)
	if !ok {
		return
	}
	if err := controller.service.Cancel(user, ctx.Param("ticketUid")); err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (controller gatewayController) me(ctx *gin.Context) {
	user, ok := username(ctx)
	if !ok {
		return
	}
	result, err := controller.service.Me(user)
	if err != nil {
		failure(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (controller gatewayController) transfer(ctx *gin.Context) {
	log.WithContext(ctx).Debug("GatewayController: transfer: Start")
	controller.proxy.ServeHTTP(ctx.Writer, ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), gin.ContextKey, ctx)))
	log.WithContext(ctx).Debug("GatewayController: transfer: Success")
}
