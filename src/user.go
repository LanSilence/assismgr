package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	userMutex sync.Mutex
)

type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Email        string `json:"email,omitempty"`
}

type PasswordChangeRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func initUser() {
	// 初始化用户文件
	if err := initUserFile(); err != nil {
		panic("Failed to initialize user file: " + err.Error())
	}

	// 认证路由

	handleAuthRoute("/change-password", changePasswordHandler)
	handleAuthRoute("/logout", logoutHandler)

}

func initUserFile() error {
	userMutex.Lock()
	defer userMutex.Unlock()

	if _, err := os.Stat("/mnt/data/user.json"); os.IsNotExist(err) {
		defaultPass := "123456"
		hash, _ := bcrypt.GenerateFromPassword([]byte(defaultPass), bcrypt.DefaultCost)

		user := User{
			Username:     "admin",
			PasswordHash: string(hash),
		}

		data, _ := json.MarshalIndent(user, "", "  ")
		return os.WriteFile("/mnt/data/user.json", data, 0600)
	}
	return nil
}

func loadUser() (*User, error) {
	userMutex.Lock()
	defer userMutex.Unlock()

	data, err := os.ReadFile("/mnt/data/user.json")
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func saveUser(user *User) error {
	userMutex.Lock()
	defer userMutex.Unlock()

	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("/mnt/data/user.json", data, 0600)
}

func loginHandler_test(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

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
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// 生成JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.Username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
	})
}

func changePasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req PasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user, err := loadUser()
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		http.Error(w, "Invalid old password", http.StatusUnauthorized)
		return
	}

	// 生成新哈希
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to generate password", http.StatusInternalServerError)
		return
	}

	// 更新用户信息
	user.PasswordHash = string(newHash)
	if err := saveUser(user); err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Password updated successfully",
	})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	// 客户端应删除本地存储的token
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}
