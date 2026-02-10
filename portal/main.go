package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

// --- Domain Models ---
type Course struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Credits    int    `json:"credits"`
	OpenSlots  int    `json:"open_slots"`
	IsEnrolled bool   `json:"is_enrolled"`
}

type GradeRecord struct {
	CourseID string `json:"course_id"`
	Grade    string `json:"grade"`
}

type DashboardData struct {
	Username    string
	Role        string
	Courses     []Course
	Grades      []GradeRecord
	GradeError  string
	CourseError string
}

// --- HTML Templates ---
const loginHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>University Login</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@1/css/pico.min.css">
</head>
<body>
    <main class="container">
        <article style="max-width: 400px; margin: auto;">
            <header><hgroup><h2>Welcome Back</h2><h3>University Portal</h3></hgroup></header>
            <form action="/login" method="POST">
                <input type="text" name="username" placeholder="Username" required>
                <input type="password" name="password" placeholder="Password" required>
                <button type="submit" class="contrast">Log In</button>
            </form>
        </article>
    </main>
</body>
</html>
`

const dashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Dashboard</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@1/css/pico.min.css">
    <style>
        .status-down { border-left: 5px solid #e74c3c; background-color: #2c0b0e; padding: 15px; margin-bottom: 20px;}
        .course-card { padding: 10px; border-bottom: 1px solid #333; display: flex; justify-content: space-between; align-items: center; }
        .enrolled-badge { color: #2ecc71; font-weight: bold; border: 1px solid #2ecc71; padding: 5px 10px; border-radius: 4px; }
    </style>
</head>
<body>
    <nav class="container-fluid">
        <ul><li><strong>University Portal</strong></li></ul>
        <ul>
            <li>User: {{.Username}} <mark>{{.Role}}</mark></li>
            <li><a href="/logout" role="button" class="outline secondary">Logout</a></li>
        </ul>
    </nav>
    <main class="container">
        <div class="grid">

            <article>
                <header><h3>📚 Open Courses</h3></header>
                {{if .CourseError}}
                    <div class="status-down"><strong>⚠️ Course Service Offline</strong></div>
                {{else}}
                    {{range .Courses}}
                        <div class="course-card">
                            <div><strong>{{.ID}}</strong>: {{.Title}}<br><small>Slots: {{.OpenSlots}}</small></div>

                            {{/* LOGIC: Only Students can Enroll */}}
                            {{if eq $.Role "student"}}
                                {{if .IsEnrolled}}
                                    <button disabled class="outline" style="width: auto; padding: 5px 15px; font-size: 0.8rem; border-color: #2ecc71; color: #2ecc71;">✅ Enrolled</button>
                                {{else if gt .OpenSlots 0}}
                                    <form action="/enroll" method="POST" style="margin:0;">
                                        <input type="hidden" name="course_id" value="{{.ID}}">
                                        <button type="submit" style="width: auto; padding: 5px 15px; font-size: 0.8rem;">Enroll</button>
                                    </form>
                                {{else}}
                                    <button disabled style="width: auto; padding: 5px 15px; font-size: 0.8rem;">Full</button>
                                {{end}}
                            {{else}}
                                <button disabled class="outline" style="width: auto; padding: 5px 15px; font-size: 0.8rem;">View Only</button>
                            {{end}}
                        </div>
                    {{end}}
                {{end}}
            </article>

            <article>
                {{if eq .Role "student"}}
                    <header><h3>🎓 My Grades</h3></header>
                    {{if .GradeError}}
                        <div class="status-down"><strong>⚠️ Grading Service Offline</strong></div>
                    {{else}}
                        <table role="grid">
                            <thead><tr><th>Course</th><th>Grade</th></tr></thead>
                            <tbody>
                                {{range .Grades}}
                                <tr><td>{{.CourseID}}</td><td><strong>{{.Grade}}</strong></td></tr>
                                {{else}}<tr><td colspan="2">No grades recorded.</td></tr>{{end}}
                            </tbody>
                        </table>
                    {{end}}
                {{end}}

                {{if eq .Role "faculty"}}
                    <header><h3>📝 Faculty Tools</h3></header>
                    <h5>Upload New Grade</h5>
                    <form action="/upload-grade" method="POST">
                        <div class="grid">
                            <input type="text" name="student_id" placeholder="Student ID" required>
                            <input type="text" name="course_id" placeholder="Course ID" required>
                            <input type="text" name="grade" placeholder="Grade" required>
                        </div>
                        <button type="submit" class="secondary">Submit Grade</button>
                    </form>
                {{end}}
            </article>
        </div>
    </main>
</body>
</html>
`

// --- Helpers ---
func fetchFromNode(url string, token string, target interface{}) error {
	client := http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// --- Handlers ---
func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	cookieToken, err := r.Cookie("session_token")
	cookieUser, _ := r.Cookie("username")
	cookieRole, _ := r.Cookie("role")

	if err != nil || cookieUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Validate Token
	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}

	client := http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", authURL+"/validate", nil)
	req.Header.Set("Authorization", "Bearer "+cookieToken.Value)
	if resp, err := client.Do(req); err != nil || resp.StatusCode != 200 {
		http.Redirect(w, r, "/logout", http.StatusSeeOther)
		return
	}

	data := DashboardData{Username: cookieUser.Value, Role: cookieRole.Value}

	// 1. Fetch Courses (Everyone sees courses)
	courseURL := os.Getenv("COURSE_SERVICE_URL")
	if courseURL == "" {
		courseURL = "http://localhost:8082"
	}
	if err := fetchFromNode(courseURL+"/courses?student_id="+cookieUser.Value, cookieToken.Value, &data.Courses); err != nil {
		data.CourseError = "Service Unreachable"
	}

	// 2. Fetch Grades (ONLY IF STUDENT)
	// Optimization: Don't bother calling Node 4 for grades if we are Faculty
	if data.Role == "student" {
		gradeURL := os.Getenv("GRADE_SERVICE_URL")
		if gradeURL == "" {
			gradeURL = "http://localhost:8083"
		}
		if err := fetchFromNode(gradeURL+"/grades?student_id="+cookieUser.Value, cookieToken.Value, &data.Grades); err != nil {
			data.GradeError = "Service Unreachable"
		}
	}

	tmpl, _ := template.New("dash").Parse(dashboardHTML)
	tmpl.Execute(w, data)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl, _ := template.New("login").Parse(loginHTML)
		tmpl.Execute(w, nil)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}

	jsonData, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(authURL+"/login", "application/json", bytes.NewBuffer(jsonData))

	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Login Failed", http.StatusUnauthorized)
		return
	}
	defer resp.Body.Close()
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	expire := time.Now().Add(1 * time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "session_token", Value: result["token"], Path: "/", Expires: expire})
	http.SetCookie(w, &http.Cookie{Name: "username", Value: username, Path: "/", Expires: expire})
	http.SetCookie(w, &http.Cookie{Name: "role", Value: result["role"], Path: "/", Expires: expire})
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func enrollHandler(w http.ResponseWriter, r *http.Request) {
	cookieUser, _ := r.Cookie("username")
	courseURL := os.Getenv("COURSE_SERVICE_URL")
	if courseURL == "" {
		courseURL = "http://localhost:8082"
	}

	payload := map[string]string{"course_id": r.FormValue("course_id"), "student_id": cookieUser.Value}
	jsonData, _ := json.Marshal(payload)
	http.Post(courseURL+"/enroll", "application/json", bytes.NewBuffer(jsonData))
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func uploadGradeHandler(w http.ResponseWriter, r *http.Request) {
	cookieToken, _ := r.Cookie("session_token")
	gradeURL := os.Getenv("GRADE_SERVICE_URL")
	if gradeURL == "" {
		gradeURL = "http://localhost:8083"
	}

	data := map[string]string{
		"student_id": r.FormValue("student_id"),
		"course_id":  r.FormValue("course_id"),
		"grade":      r.FormValue("grade"),
	}
	jsonData, _ := json.Marshal(data)

	client := http.Client{}
	req, _ := http.NewRequest("POST", gradeURL+"/upload-grade", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+cookieToken.Value)
	client.Do(req)

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "session_token", MaxAge: -1, Path: "/"})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func main() {
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/dashboard", dashboardHandler)
	http.HandleFunc("/enroll", enrollHandler)
	http.HandleFunc("/upload-grade", uploadGradeHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/login", http.StatusSeeOther) })

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Node 1 (Portal) running on port %s...\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
