package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const AuthValidateURL = "http://node_auth:8081/validate"

type GradeRecord struct {
	StudentID string `json:"student_id"`
	CourseID  string `json:"course_id"`
	Grade     string `json:"grade"`
}

type AuthResponse struct {
	Status   string `json:"status"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

var gradeBook = []GradeRecord{
	{StudentID: "student1", CourseID: "CCPROG1", Grade: "4.0"},
	{StudentID: "student1", CourseID: "MTH101A", Grade: "3.5"},
	{StudentID: "student2", CourseID: "CCPROG1", Grade: "2.0"},
}

func validateTokenAndGetUser(tokenString string) (*AuthResponse, bool) {
	client := http.Client{Timeout: 2 * time.Second}

	req, _ := http.NewRequest("GET", AuthValidateURL, nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil, false
	}
	defer resp.Body.Close()

	var authData AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authData); err != nil {
		return nil, false
	}

	return &authData, true
}

func getGrades(w http.ResponseWriter, r *http.Request) {
	// 1. EXTRACT TOKEN
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Missing token", http.StatusUnauthorized)
		return
	}
	tokenValue := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. IDENTIFY USER (Talk to Auth Service)
	user, valid := validateTokenAndGetUser(tokenValue)
	if !valid {
		http.Error(w, "Unauthorized: Invalid Token", http.StatusUnauthorized)
		return
	}

	// 3. AUTHORIZATION CHECK (The Logic You Asked For)
	requestedStudent := r.URL.Query().Get("student_id")

	// RULE: You can only see the data if:
	// A) You are a Faculty member
	// OR
	// B) You are the student requesting your own data
	if user.Role != "faculty" && user.Username != requestedStudent {
		http.Error(w, "Forbidden: You cannot view another student's grades", http.StatusForbidden)
		return
	}

	// 4. Return Data
	var results []GradeRecord
	for _, rec := range gradeBook {
		if rec.StudentID == requestedStudent {
			results = append(results, rec)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func uploadGrade(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenValue := strings.TrimPrefix(authHeader, "Bearer ")

	user, valid := validateTokenAndGetUser(tokenValue)
	if !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// RULE: Only Faculty can upload
	if user.Role != "faculty" {
		http.Error(w, "Forbidden: Only faculty can upload grades", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var newGrade GradeRecord
	if err := json.NewDecoder(r.Body).Decode(&newGrade); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	gradeBook = append(gradeBook, newGrade)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status": "grade recorded"}`))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/grades", getGrades)
	mux.HandleFunc("/upload-grade", uploadGrade)

	fmt.Println("Node 4 (Grade Service) running on port 8083...")
	log.Fatal(http.ListenAndServe("0.0.0.0:8083", mux))
}
