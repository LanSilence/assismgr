package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// 用户模型（示例数据）
var users = map[string]string{
	"admin": "123456", // 实际应使用 bcrypt 哈希存储
}

// JWT 配置
var (
	jwtSecret = []byte("your-secret-key") // 生产环境应从安全配置读取
	tokenExp  = time.Hour * 1             // Token 有效期
)

// 登录请求结构体
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// 登录响应结构体
type LoginResponse struct {
	Token string `json:"token"`
}

// 受保护接口响应结构体
type ProfileResponse struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func initLogin() {
	// 注册路由
	http.HandleFunc("/login", loginHandler)
}

// 登录处理函数
func loginHandler(w http.ResponseWriter, r *http.Request) {

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	// 只接受 POST 请求
	if r.Method != http.MethodPost {
		http.ServeFile(w, r, *staticFileDir+"/login.html")
		return
	}
	// 解析请求体
	// var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	// 验证用户凭证

	user, err := loadUser()
	if err != nil {
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	if user.Username != creds.Username {
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password)); err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}

	// 生成 JWT Token
	token, err := generateToken(creds.Username)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
		return
	}

	// 返回 Token
	respondJSON(w, http.StatusOK, LoginResponse{Token: token})
}

// 生成 JWT Token
func generateToken(username string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExp)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// 验证 Token
func validateToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		},
	)

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}

// 通用 JSON 响应工具函数
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
