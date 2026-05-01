package main

import (
	"fmt"
	"log"

	"recipe-server/config"
	"recipe-server/internal/handler"
	"recipe-server/internal/middleware"
	"recipe-server/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	if err := config.Load("config.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(config.AppConfig.MySQL.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// 自动迁移
	db.AutoMigrate(
		&model.Family{},
		&model.User{},
		&model.FamilyMember{},
		&model.Recipe{},
		&model.Menu{},
		&model.MenuItem{},
		&model.Favorite{},
	)

	// 路由
	r := gin.Default()

	// 静态文件（上传图片访问）
	r.Static("/uploads", "/www/uploads")

	api := r.Group("/api")
	{
		// 公开接口
		authH := handler.NewAuthHandler(db)
		api.POST("/auth/login", authH.Login)

		// 需要认证
		auth := api.Group("", middleware.AuthRequired())
		{
			// 用户
			auth.GET("/users/me", authH.GetProfile)
			auth.PUT("/users/me", authH.UpdateProfile)

			// 家庭
			familyH := handler.NewFamilyHandler(db)
			auth.POST("/families", familyH.Create)
			auth.POST("/families/join", familyH.Join)
			auth.GET("/families", familyH.List)
			auth.GET("/families/:id/members", familyH.Members)

			// 菜谱
			recipeH := handler.NewRecipeHandler(db)
			auth.POST("/recipes", recipeH.Create)
			auth.PUT("/recipes/:id", recipeH.Update)
			auth.DELETE("/recipes/:id", recipeH.Delete)
			auth.GET("/recipes/:id", recipeH.Get)
			auth.GET("/recipes", recipeH.List)
			auth.POST("/recipes/:id/cooked", recipeH.Cooked)

			// 点菜
			menuH := handler.NewMenuHandler(db)
			auth.POST("/menus", menuH.Create)
			auth.GET("/menus/:id", menuH.Get)
			auth.GET("/menus", menuH.List)
			auth.POST("/menus/:id/items", menuH.AddItem)
			auth.DELETE("/menus/:id/items/:item_id", menuH.RemoveItem)
			auth.POST("/menus/:id/confirm", menuH.Confirm)

			// 收藏
			favH := handler.NewFavoriteHandler(db)
			auth.POST("/favorites/:id", favH.Add)
			auth.DELETE("/favorites/:id", favH.Remove)
			auth.GET("/favorites", favH.List)

			// 上传
			uploadH := &handler.UploadHandler{}
			auth.POST("/upload", uploadH.Upload)

			// AI
			aiH := handler.NewAIHandler(db)
			auth.POST("/ai/recommend", aiH.Recommend)
		}
	}

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("服务启动: http://localhost%s", addr)
	r.Run(addr)
}
