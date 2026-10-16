package main

import (
	"fmt"

	coreConfig "github.com/CakeForKit/rsoi-lab2/lb-core/config"
	"github.com/CakeForKit/rsoi-lab2/lb-core/server"
	"github.com/CakeForKit/rsoi-lab2/lb-core/utils"
	gatewayConfig "github.com/CakeForKit/rsoi-lab2/ms-gateway/internal/config"
	"github.com/CakeForKit/rsoi-lab2/ms-gateway/internal/controller"
	"github.com/gin-gonic/gin"
)

func main() {
	utils.InitLogger()

	utils.CheckedError(coreConfig.Load(&coreConfig.CoreConfig))
	utils.CheckedError(coreConfig.Load(&gatewayConfig.AppConfig))

	fmt.Printf("config: %d\n\n", coreConfig.CoreConfig.Port)

	utils.CheckedError(server.NewHttpServer(func(router *gin.Engine) {
		utils.RegisterController(router, controller.GetGatewayController)
	}).Run())
}
