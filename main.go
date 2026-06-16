// Package main 菜谱服务入口。
// 基于 Gin + GORM 构建，提供用户认证（微信小程序登录）、
// 家庭管理、菜谱 CRUD、每日点菜、收藏、图片上传以及 AI 智能推荐等 API。
package main

import (
	"context"
	"fmt"
	"log"

	"recipe-server/config"
	"recipe-server/internal/cache"
	"recipe-server/internal/handler"
	"recipe-server/internal/middleware"
	"recipe-server/internal/model"
	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// main 应用入口：加载配置 → 连接数据库 → 自动迁移 → 注册路由 → 启动服务。
func main() {
	// 1. 加载 YAML 配置文件
	if err := config.Load("config.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 连接 MySQL 数据库（GORM）
	db, err := gorm.Open(mysql.Open(config.AppConfig.MySQL.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// 3. 自动迁移：根据模型结构体创建/更新表结构
	if err := db.AutoMigrate(
		&model.Family{},
		&model.User{},
		&model.FamilyMember{},
		&model.RecipeCategory{},
		&model.Recipe{},
		&model.CatalogRecipe{},
		&model.DailyOrder{},
		&model.Favorite{},
		&model.Notification{},
		&model.NotificationDelivery{},
		&model.NotificationChannel{},
		&model.FridgeItem{},
		&model.FridgeScan{},
	); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if err := service.NewCategoryService(db).SyncAllFamilies(); err != nil {
		log.Printf("警告: 同步菜谱分类失败: %v", err)
	}

	wsHub := service.NewWebSocketHub()
	imageWorkerHub := service.NewImageWorkerHub(nil)
	imageWorkerSvc := service.NewImageWorkerService(db, imageWorkerHub)
	fridgeSvc := service.NewFridgeService(db, imageWorkerSvc)
	imageWorkerSvc.SetFridgeRecognizer(fridgeSvc)
	fridgeH := handler.NewFridgeHandler(fridgeSvc)
	notifySvc := service.NewNotificationService(db, wsHub)
	wsHub.SetOnConnect(func(userID uint64) {
		notifySvc.FlushUnreadWebSocket(userID)
	})
	if config.AppConfig.Notification.Worker.Enabled {
		notifySvc.StartRetryWorker()
	}

	redisCache := cache.NewRedisCache(
		config.AppConfig.Redis.Addr,
		config.AppConfig.Redis.Password,
		config.AppConfig.Redis.DB,
	)
	if err := redisCache.Ping(context.Background()); err != nil {
		log.Printf("警告: Redis 连接失败（AI推荐/天气缓存不可用）: %v", err)
	}
	weatherSvc := service.NewWeatherService(redisCache, nil)
	aiSvc := service.NewAIService()
	rateLimitSvc := service.NewAIRateLimitService(redisCache)
	aiCtxSvc := service.NewAIContextService(db, weatherSvc)
	aiRecommendSvc := service.NewAIRecommendService(db, redisCache, aiSvc, aiCtxSvc, rateLimitSvc)
	catalogSvc := service.NewCatalogRecipeService(db, aiSvc, rateLimitSvc)
	aiH := handler.NewAIHandler(aiRecommendSvc, weatherSvc)
	catalogH := handler.NewCatalogRecipeHandler(catalogSvc)

	// 4. 创建 Gin 路由引擎（带默认中间件：Logger + Recovery）
	r := gin.Default()

	// 静态文件服务：上传的图片可通过 /uploads 路径直接访问
	r.Static("/uploads", "/www/uploads")

	// API 路由组
	api := r.Group("/api")
	{
		// ---------- 公开接口（无需登录） ----------
		authH := handler.NewAuthHandler(db)
		api.POST("/auth/login", authH.Login) // 微信登录

		recipeH := handler.NewRecipeHandler(db)
		recipeBrowse := api.Group("", middleware.OptionalAuth())
		recipeBrowse.GET("/recipes", recipeH.List)       // 菜谱列表（本家 + 公开）
		recipeBrowse.GET("/recipes/:id", recipeH.Get)   // 菜谱详情（本家或公开）
		api.GET("/weather", aiH.Weather)      // 天气（默认成都）
		api.GET("/app/features", handler.NewAppHandler().Features) // 功能开关（公开）

		categoryH := handler.NewCategoryHandler(db)
		api.GET("/categories/public", categoryH.ListPublic) // 公开菜谱分类（无需登录）

		// ---------- 需要认证的接口 ----------
		auth := api.Group("", middleware.AuthRequired())
		{
			// 用户信息
			auth.GET("/users/me", authH.GetProfile)      // 获取个人信息
			auth.PUT("/users/me", authH.UpdateProfile)    // 更新个人信息

			// 家庭管理
			familyH := handler.NewFamilyHandler(db)
			auth.POST("/families", familyH.Create)              // 创建家庭
			auth.POST("/families/join", familyH.Join)           // 通过邀请码加入家庭
			auth.GET("/families", familyH.List)                 // 我的家庭列表
			auth.GET("/families/:id/members", familyH.Members)  // 家庭成员列表
			auth.POST("/families/chef", familyH.ToggleChef)     // 切换厨师身份

			auth.GET("/categories", categoryH.List)          // 菜谱分类列表

			// 菜谱写操作（需登录）
			auth.POST("/recipes", recipeH.Create)            // 创建菜谱
			auth.PUT("/recipes/:id", recipeH.Update)         // 更新菜谱
			auth.DELETE("/recipes/:id", recipeH.Delete)      // 删除菜谱
			auth.POST("/recipes/:id/cooked", recipeH.Cooked) // 标记已烹饪

			// 每日点菜
			orderH := handler.NewOrderHandler(db, wsHub)
			auth.GET("/orders", orderH.List)         // 查看点菜列表
			auth.POST("/orders", orderH.Add)         // 点一道菜
			auth.DELETE("/orders/:id", orderH.Remove) // 取消点菜
			auth.POST("/orders/share", orderH.Share) // 创建动态消息分享

			// 厨师通知
			notifyH := handler.NewNotificationHandler(db, wsHub)
			auth.GET("/notifications/unread", notifyH.ListUnread)
			auth.POST("/notifications/:id/read", notifyH.MarkRead)
			chH := handler.NewNotificationChannelHandler(db)
			auth.GET("/notification-channels", chH.List)
			auth.POST("/notification-channels", chH.Create)
			auth.PUT("/notification-channels/:id", chH.Update)
			auth.DELETE("/notification-channels/:id", chH.Delete)

			// 收藏
			favH := handler.NewFavoriteHandler(db)
			auth.POST("/favorites/:id", favH.Add)     // 收藏菜谱
			auth.DELETE("/favorites/:id", favH.Remove) // 取消收藏
			auth.GET("/favorites", favH.List)          // 收藏列表

			// 图片上传
			uploadH := &handler.UploadHandler{ImageWorker: imageWorkerSvc}
			auth.POST("/upload", uploadH.Upload)

			// AI 智能推荐
			auth.POST("/ai/recommend", aiH.Recommend)
			auth.GET("/ai/items/:item_id", aiH.GetItem)
			auth.POST("/ai/items/:item_id/import-recipe", aiH.ImportRecipe)
			auth.POST("/ai/items/:item_id/add-order", aiH.AddOrder)

			// 全局菜谱库（搜索/生成）
			auth.POST("/catalog-recipes/lookup", catalogH.Lookup)
			auth.GET("/catalog-recipes/:id", catalogH.Get)
			auth.POST("/catalog-recipes/:id/use", catalogH.Use)

			// 冰箱食材
			auth.GET("/fridge/items", fridgeH.ListItems)
			auth.POST("/fridge/items", fridgeH.CreateItems)
			auth.PUT("/fridge/items/:id", fridgeH.UpdateItem)
			auth.DELETE("/fridge/items/:id", fridgeH.DeleteItem)
			auth.POST("/fridge/scans", fridgeH.CreateScan)
			auth.GET("/fridge/scans/:id", fridgeH.GetScan)
			auth.POST("/fridge/scans/:id/confirm", fridgeH.ConfirmScan)
		}
	}

	// WebSocket（需 JWT query token）
	wsPath := config.AppConfig.Notification.WebSocket.Path
	if wsPath == "" {
		wsPath = "/api/ws"
	}
	r.GET(wsPath, wsHub.HandleWebSocket)

	if config.AppConfig.ImageWorker.Enabled {
		iwPath := config.AppConfig.ImageWorker.Path
		if iwPath == "" {
			iwPath = "/api/ws/image-worker"
		}
		r.GET(iwPath, imageWorkerHub.HandleWebSocket)
		log.Printf("ImageWorker WebSocket: %s (WSS via nginx)", iwPath)
	}

	// 启动 HTTP 服务器
	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("服务启动: http://localhost%s", addr)
	r.Run(addr)
}
