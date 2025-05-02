// internal/api/middleware.go
package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
)

// 定义JWT密钥和过期时间
var (
	jwtSecret     = []byte("your-secret-key") // 在生产环境中应该从配置或环境变量中获取
	jwtExpiration = 24 * time.Hour
)

// Claims 定义JWT的声明
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// RateLimiter 使用令牌桶算法实现速率限制
type RateLimiter struct {
	tokens     map[string]float64 // 每个IP的令牌数
	lastUpdate map[string]time.Time
	rate       float64 // 令牌生成速率（每秒）
	capacity   float64 // 令牌桶容量
	mu         sync.Mutex
}

var (
	limiter *RateLimiter
	rdb     *redis.Client
)

func init() {
	// 初始化速率限制器
	limiter = &RateLimiter{
		tokens:     make(map[string]float64),
		lastUpdate: make(map[string]time.Time),
		rate:       1,     // 每秒生成1个令牌
		capacity:   10,    // 最多存储10个令牌
	}

	// 初始化Redis客户端
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis地址
		Password: "",               // 密码
		DB:       0,                // 使用默认DB
	})
}

// LoggingMiddleware 记录请求日志
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		endTime := time.Now()
		latency := endTime.Sub(startTime)

		log.Printf("[GIN] %v | %3d | %13v | %15s | %s | %s",
			endTime.Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			c.Request.Method,
			c.Request.RequestURI,
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

// AuthMiddleware 实现JWT认证
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "未提供认证令牌"})
			return
		}

		// 解析Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(401, gin.H{"error": "无效的认证格式"})
			return
		}

		// 解析JWT token
		token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "无效的令牌"})
			return
		}

		// 验证token
		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// 将用户信息存储在上下文中
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Next()
		} else {
			c.AbortWithStatusJSON(401, gin.H{"error": "无效的令牌声明"})
			return
		}
	}
}

// RateLimitMiddleware 实现基于Redis的分布式速率限制
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s", ip)
		ctx := context.Background()

		// 使用Redis实现滑动窗口速率限制
		now := time.Now().Unix()
		windowSize := int64(60) // 1分钟的窗口
		maxRequests := int64(60) // 每分钟最大请求数

		// 使用管道执行原子操作
		pipe := rdb.Pipeline()
		pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", now-windowSize))
		pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: now})
		pipe.ZCard(ctx, key)
		pipe.Expire(ctx, key, time.Minute)
		
		res, err := pipe.Exec(ctx)
		if err != nil {
			// Redis错误时降级为内存限流
			if !limiter.Allow(ip) {
				c.AbortWithStatusJSON(429, gin.H{"error": "请求过于频繁"})
				return
			}
		} else {
			count := res[2].(*redis.IntCmd).Val()
			if count > maxRequests {
				c.AbortWithStatusJSON(429, gin.H{"error": "请求过于频繁"})
				return
			}
		}

		c.Next()
	}
}

// ErrorHandlerMiddleware 全局错误处理
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

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

// GenerateToken 生成JWT令牌
func GenerateToken(userID, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Allow 检查是否允许请求通过
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	lastUpdate, exists := rl.lastUpdate[ip]
	if !exists {
		rl.tokens[ip] = rl.capacity
		rl.lastUpdate[ip] = now
		return true
	}

	// 计算新生成的令牌
	elapsed := now.Sub(lastUpdate).Seconds()
	newTokens := elapsed * rl.rate
	currentTokens := rl.tokens[ip] + newTokens
	if currentTokens > rl.capacity {
		currentTokens = rl.capacity
	}

	// 如果没有足够的令牌，拒绝请求
	if currentTokens < 1 {
		return false
	}

	// 消耗一个令牌
	rl.tokens[ip] = currentTokens - 1
	rl.lastUpdate[ip] = now
	return true
}
