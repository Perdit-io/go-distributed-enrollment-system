package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func getJWTKey() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return []byte("fallback_secret_for_local_testing")
	}
	return []byte(secret)
}

// --- Models ---
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// --- Data ---
var users = map[string]string{
	"student1": "pass123",
	"student2": "pass123",
	"faculty1": "pass123",
}
var roles = map[string]string{
	"student1": "student",
	"student2": "student",
	"faculty1": "faculty",
}

func login(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	expectedPassword, ok := users[creds.Username]
	if !ok || expectedPassword != creds.Password {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Token valid for 1 HOUR
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		Username: creds.Username,
		Role:     roles[creds.Username],
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(getJWTKey())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"token": "` + tokenString + `", "role": "` + roles[creds.Username] + `"}`))
}

func validate(w http.ResponseWriter, r *http.Request) {
	// 1. Get token from Header (Authorization: Bearer <token>)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. Parse and Validate
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return getJWTKey(), nil
	})

	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized) // Token expired or invalid
		return
	}

	// 3. Token is good
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "valid", "username": "` + claims.Username + `", "role": "` + claims.Role + `"}`))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", login)
	mux.HandleFunc("/validate", validate) // Register the new route

	fmt.Println("Node 2 (Auth Service) running on port 8081...")
	log.Fatal(http.ListenAndServe("0.0.0.0:8081", mux))
}
