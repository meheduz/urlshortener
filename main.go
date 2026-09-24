package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	alphabet   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength = 5
	maxRetries = 5
)

type app struct {
	db      *sql.DB
	baseURL string
}

func main() {
	addr := env("ADDR", ":8080")
	baseURL := strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/")
	dbPath := env("DB_PATH", "data/urls.db")

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	a := &app{db: db, baseURL: baseURL}

	log.Printf("url shortener listening on %s (base url %s)", addr, baseURL)
	if err := http.ListenAndServe(addr, a.routes()); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
		code TEXT PRIMARY KEY,
		original_url TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", a.handleShorten)
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /{code}", a.handleRedirect)
	return mux
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) handleShorten(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if !validURL(req.URL) {
		writeError(w, http.StatusBadRequest, "url must be a valid http or https url")
		return
	}

	code, err := a.store(req.URL)
	if err != nil {
		log.Printf("store url: %v", err)
		writeError(w, http.StatusInternalServerError, "could not store url")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"short_url": a.baseURL + "/" + code})
}

func (a *app) handleRedirect(w http.ResponseWriter, r *http.Request) {
	var original string
	err := a.db.QueryRow(`SELECT original_url FROM urls WHERE code = ?`, r.PathValue("code")).Scan(&original)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "unknown code")
		return
	}
	if err != nil {
		log.Printf("lookup code: %v", err)
		writeError(w, http.StatusInternalServerError, "lookup failed")
		return
	}
	http.Redirect(w, r, original, http.StatusFound)
}

// store inserts the url under a fresh random code, retrying on the rare
// collision with an existing code.
func (a *app) store(original string) (string, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		code, err := generateCode()
		if err != nil {
			return "", err
		}
		_, err = a.db.Exec(`INSERT INTO urls (code, original_url) VALUES (?, ?)`, code, original)
		if err == nil {
			return code, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func generateCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
