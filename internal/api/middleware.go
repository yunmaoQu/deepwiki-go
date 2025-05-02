// internal/api/middleware.go
package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggingMiddleware 记录请求日志
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 结束时间
		endTime := time.Now()

		// 执行时间
		latency := endTime.Sub(startTime)

		// 请求方法
		reqMethod := c.Request.Method

		// 请求路由
		reqURI := c.Request.RequestURI

		// 状态码
		statusCode := c.Writer.Status()

		// 客户端IP
		clientIP := c.ClientIP()

		// 日志格式
		log.Printf("[GIN] %v | %3d | %13v | %15s | %s | %s",
			endTime.Format("2006/01/02 - 15:04:05"),
			statusCode,
			latency,
			clientIP,
			reqMethod,
			reqURI,
		)
	}
}

// CORSMiddleware 处理跨域请求
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// AuthMiddleware 处理API认证
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 这里应该根据应用需求实现认证逻辑
		// 例如，可以检查请求头中的API密钥
		/*
			apiKey := c.GetHeader("Authorization")
			if apiKey == "" {
				c.AbortWithStatusJSON(401, gin.H{
					"error": "缺少认证信息",
				})
				return
			}

			// 验证API密钥
			if !isValidAPIKey(apiKey) {
				c.AbortWithStatusJSON(401, gin.H{
					"error": "认证失败",
				})
				return
			}
		*/

		// 开发阶段暂时跳过认证
		c.Next()
	}
}

// RateLimitMiddleware 实现请求速率限制
func RateLimitMiddleware() gin.HandlerFunc {
	// 这里可以使用令牌桶或漏桶算法实现速率限制
	// 简单实现，实际应使用更健壮的解决方案如Redis
	return func(c *gin.Context) {
		// 在这里实现速率限制逻辑
		// 例如，可以限制每个IP每分钟的请求数量
		// 简单起见，这里只是一个占位符
		c.Next()
	}
}

// ErrorHandlerMiddleware 全局错误处理
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			// 记录错误
			for _, err := range c.Errors {
				log.Printf("API错误: %v", err.Err)
			}

			// 返回最后一个错误给客户端
			lastErr := c.Errors.Last()
			c.JSON(c.Writer.Status(), gin.H{
				"error": lastErr.Error(),
			})
		}
	}
}
