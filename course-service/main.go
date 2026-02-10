package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

// --- Domain Models ---
type Course struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Credits    int    `json:"credits"`
	OpenSlots  int    `json:"open_slots"`
	IsEnrolled bool   `json:"is_enrolled"`
}

type EnrollRequest struct {
	CourseID  string `json:"course_id"`
	StudentID string `json:"student_id"`
}

// --- In-Memory Database ---
var (
	mu          sync.Mutex
	enrollments = make(map[string]bool) // Key: "CourseID:StudentID"

	// Define courses as pointers so we can modify them easily in the loop
	courses = []*Course{
		{ID: "CCPROG2", Title: "Programming with Structured Data Types", Credits: 3, OpenSlots: 20},
		{ID: "STDISCM", Title: "Distributed Computing", Credits: 4, OpenSlots: 15},
		{ID: "CSMATH1", Title: "Differential Calculus for Computer Science Students", Credits: 3, OpenSlots: 30},
	}
)

// --- Handlers ---

func getCourses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check who is asking
	studentID := r.URL.Query().Get("student_id")

	mu.Lock()
	defer mu.Unlock()

	// Dynamic Response: Calculate 'IsEnrolled' for this specific student
	// We create a temporary list so we don't mess up the global state for other users
	var responseList []Course
	for _, c := range courses {
		tempCourse := *c // Copy value
		if studentID != "" {
			// Check if this student is in the map
			if enrollments[c.ID+":"+studentID] {
				tempCourse.IsEnrolled = true
			}
		}
		responseList = append(responseList, tempCourse)
	}

	json.NewEncoder(w).Encode(responseList)
}

func enroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// 1. Check Duplication
	enrollKey := req.CourseID + ":" + req.StudentID
	if enrollments[enrollKey] {
		http.Error(w, "Student already enrolled", http.StatusConflict)
		return
	}

	// 2. Find Course & Decrement
	for _, c := range courses {
		if c.ID == req.CourseID {
			if c.OpenSlots > 0 {
				c.OpenSlots--
				enrollments[enrollKey] = true

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "enrolled"}`))
				return
			}
			http.Error(w, "Course full", http.StatusConflict)
			return
		}
	}
	http.Error(w, "Course not found", http.StatusNotFound)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/courses", getCourses)
	mux.HandleFunc("/enroll", enroll)

	fmt.Printf("Node 3 (Course Service) running on port %s...\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
