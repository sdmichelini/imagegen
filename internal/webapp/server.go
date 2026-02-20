package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

var hashedDistAssetPattern = regexp.MustCompile(`^[a-z0-9-]+-[A-Z0-9]{6,}\.(js|css|png|jpg|jpeg|webp|svg|ico)$`)

type Server struct {
	store         *Store
	templates     *template.Template
	manifestMu    sync.RWMutex
	assetManifest map[string]string
	logger        *log.Logger
	staticFS      http.FileSystem
}

type PageData struct {
	Title       string
	CurrentPath string
	Flash       string
	Brands      []Brand
	Brand       Brand
	Projects    []Project
	Project     Project
	WorkItems   []WorkItem
	WorkItem    WorkItem
	WorkImages  []WorkItemImage
	Jobs        []Job
	Job         Job
	Error       string
}

func NewServer(dataRoot string) (*Server, error) {
	store, err := NewStore(dataRoot)
	if err != nil {
		return nil, err
	}
	tmpl, err := loadTemplates("web/templates")
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:         store,
		templates:     tmpl,
		assetManifest: map[string]string{},
		logger:        log.New(os.Stdout, "[imagegen-web] ", log.LstdFlags),
		staticFS:      http.Dir("web/static"),
	}
	s.loadManifest(filepath.Join("web", "static", "dist", "manifest.json"))
	go s.jobWorkerLoop()
	return s, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", s.staticHandler()))

	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /about", s.handleAbout)
	mux.HandleFunc("GET /brands", s.handleBrands)
	mux.HandleFunc("POST /brands", s.handleCreateBrand)
	mux.HandleFunc("GET /brands/{slug}", s.handleBrandEdit)
	mux.HandleFunc("POST /brands/{slug}", s.handleUpdateBrand)

	mux.HandleFunc("GET /projects", s.handleProjects)
	mux.HandleFunc("POST /projects", s.handleCreateProject)
	mux.HandleFunc("GET /projects/{slug}", s.handleProjectDetail)
	mux.HandleFunc("POST /projects/{slug}/work-items", s.handleCreateWorkItem)
	mux.HandleFunc("GET /projects/{slug}/work-items/{itemSlug}", s.handleWorkItemDetail)
	mux.HandleFunc("POST /projects/{slug}/work-items/{itemSlug}/prompt", s.handleUpdateWorkItemPrompt)
	mux.HandleFunc("POST /projects/{slug}/work-items/{itemSlug}/generate", s.handleGenerateWorkItem)

	mux.HandleFunc("GET /jobs", s.handleJobs)
	mux.HandleFunc("GET /jobs/{jobID}", s.handleJobDetail)
	mux.HandleFunc("GET /images/{imageID}", s.handleImageByID)
	mux.HandleFunc("GET /api/jobs/{jobID}", s.handleAPIJobStatus)

	return s.loggingMiddleware(mux)
}

func (s *Server) staticHandler() http.Handler {
	fileServer := http.FileServer(s.staticFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := path.Base(r.URL.Path)
		if strings.HasPrefix(r.URL.Path, "dist/") && hashedDistAssetPattern.MatchString(filename) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	projects, _ := s.store.ListProjects()
	brands, _ := s.store.ListBrands()
	jobs, _ := s.store.ListJobs(8)
	s.render(w, r, "dashboard", PageData{
		Title:       "Dashboard",
		CurrentPath: r.URL.Path,
		Projects:    projects,
		Brands:      brands,
		Jobs:        jobs,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "about", PageData{
		Title:       "About",
		CurrentPath: r.URL.Path,
	})
}

func (s *Server) handleBrands(w http.ResponseWriter, r *http.Request) {
	brands, err := s.store.ListBrands()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "brands", PageData{
		Title:       "Brands",
		CurrentPath: r.URL.Path,
		Brands:      brands,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleCreateBrand(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	content := strings.TrimSpace(r.FormValue("content"))
	if _, err := s.store.CreateBrand(name, content); err != nil {
		brands, _ := s.store.ListBrands()
		s.render(w, r, "brands", PageData{
			Title:       "Brands",
			CurrentPath: "/brands",
			Brands:      brands,
			Error:       err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/brands?ok=Brand+created", http.StatusSeeOther)
}

func (s *Server) handleBrandEdit(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	brand, err := s.store.GetBrand(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "brand-edit", PageData{
		Title:       fmt.Sprintf("Brand: %s", brand.Slug),
		CurrentPath: "/brands",
		Brand:       brand,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleUpdateBrand(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(r.FormValue("content"))
	brand, err := s.store.UpdateBrand(slug, content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/brands/"+brand.Slug+"?ok=Brand+saved", http.StatusSeeOther)
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	brands, _ := s.store.ListBrands()
	s.render(w, r, "projects", PageData{
		Title:       "Projects",
		CurrentPath: r.URL.Path,
		Projects:    projects,
		Brands:      brands,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	defaultBrand := strings.TrimSpace(r.FormValue("default_brand"))
	project, err := s.store.CreateProject(name, defaultBrand)
	if err != nil {
		projects, _ := s.store.ListProjects()
		brands, _ := s.store.ListBrands()
		s.render(w, r, "projects", PageData{
			Title:       "Projects",
			CurrentPath: "/projects",
			Projects:    projects,
			Brands:      brands,
			Error:       err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/projects/"+project.Slug+"?ok=Project+created", http.StatusSeeOther)
}

func (s *Server) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	project, err := s.store.GetProject(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	items, _ := s.store.ListWorkItems(slug)
	brands, _ := s.store.ListBrands()
	s.render(w, r, "project-detail", PageData{
		Title:       fmt.Sprintf("Project: %s", project.Name),
		CurrentPath: "/projects",
		Project:     project,
		WorkItems:   items,
		Brands:      brands,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleCreateWorkItem(w http.ResponseWriter, r *http.Request) {
	projectSlug := r.PathValue("slug")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	itemType := strings.TrimSpace(r.FormValue("type"))
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	brandOverride := strings.TrimSpace(r.FormValue("brand_override"))
	item, err := s.store.CreateWorkItem(projectSlug, name, itemType, prompt, brandOverride)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/projects/"+Slugify(projectSlug)+"/work-items/"+item.Slug+"?ok=Work+item+created", http.StatusSeeOther)
}

func (s *Server) handleWorkItemDetail(w http.ResponseWriter, r *http.Request) {
	s.renderWorkItemPage(w, r, r.PathValue("slug"), r.PathValue("itemSlug"), "")
}

func (s *Server) handleUpdateWorkItemPrompt(w http.ResponseWriter, r *http.Request) {
	projectSlug := r.PathValue("slug")
	itemSlug := r.PathValue("itemSlug")
	if err := r.ParseForm(); err != nil {
		s.renderWorkItemPage(w, r, projectSlug, itemSlug, "invalid form")
		return
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if _, err := s.store.UpdateWorkItemPrompt(projectSlug, itemSlug, prompt); err != nil {
		s.renderWorkItemPage(w, r, projectSlug, itemSlug, err.Error())
		return
	}
	http.Redirect(w, r, "/projects/"+Slugify(projectSlug)+"/work-items/"+Slugify(itemSlug)+"?ok=Prompt+saved", http.StatusSeeOther)
}

func (s *Server) handleGenerateWorkItem(w http.ResponseWriter, r *http.Request) {
	projectSlug := Slugify(r.PathValue("slug"))
	itemSlug := Slugify(r.PathValue("itemSlug"))
	if err := r.ParseForm(); err != nil {
		s.renderWorkItemPage(w, r, projectSlug, itemSlug, "invalid form")
		return
	}
	count := 2
	if raw := strings.TrimSpace(r.FormValue("count")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			s.renderWorkItemPage(w, r, projectSlug, itemSlug, "count must be >= 1")
			return
		}
		count = v
	}
	payload := GenerateJobPayload{
		Model:        strings.TrimSpace(r.FormValue("model")),
		Count:        count,
		OutputFormat: strings.TrimSpace(r.FormValue("output_format")),
		ImageSize:    strings.TrimSpace(r.FormValue("image_size")),
		AspectRatio:  strings.TrimSpace(r.FormValue("aspect_ratio")),
		Adjustment:   strings.TrimSpace(r.FormValue("adjustment")),
	}
	job, err := s.store.CreateGenerateJob(projectSlug, itemSlug, payload)
	if err != nil {
		if wantsJSON(r) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		s.renderWorkItemPage(w, r, projectSlug, itemSlug, err.Error())
		return
	}
	if wantsJSON(r) {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"job_id":         job.ID,
			"status":         job.Status,
			"project_slug":   job.ProjectSlug,
			"work_item_slug": job.WorkItemSlug,
		})
		return
	}
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(job.ID, 10)+"?ok=Job+queued", http.StatusSeeOther)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.ListJobs(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "jobs", PageData{
		Title:       "Jobs",
		CurrentPath: "/jobs",
		Jobs:        jobs,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(r.PathValue("jobID"), 10, 64)
	if err != nil || jobID < 1 {
		http.NotFound(w, r)
		return
	}
	job, err := s.store.GetJob(jobID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	images, _ := s.store.ListJobImages(jobID)
	s.render(w, r, "job-detail", PageData{
		Title:       fmt.Sprintf("Job #%d", jobID),
		CurrentPath: "/jobs",
		Job:         job,
		WorkImages:  images,
		Flash:       r.URL.Query().Get("ok"),
	})
}

func (s *Server) handleImageByID(w http.ResponseWriter, r *http.Request) {
	imageID, err := strconv.ParseInt(r.PathValue("imageID"), 10, 64)
	if err != nil || imageID < 1 {
		http.NotFound(w, r)
		return
	}
	imagePath, err := s.store.ImagePathByID(imageID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, imagePath)
}

func (s *Server) handleAPIJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(r.PathValue("jobID"), 10, 64)
	if err != nil || jobID < 1 {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "job not found"})
		return
	}
	job, err := s.store.GetJob(jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "job not found"})
		return
	}
	images, _ := s.store.ListJobImages(jobID)
	payload := map[string]any{
		"id":             job.ID,
		"status":         job.Status,
		"error_message":  job.ErrorMessage,
		"project_slug":   job.ProjectSlug,
		"work_item_slug": job.WorkItemSlug,
		"created_at":     job.CreatedAt.Format(time.RFC3339Nano),
		"images":         images,
	}
	if job.StartedAt != nil {
		payload["started_at"] = job.StartedAt.Format(time.RFC3339Nano)
	}
	if job.FinishedAt != nil {
		payload["finished_at"] = job.FinishedAt.Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) renderWorkItemPage(w http.ResponseWriter, r *http.Request, projectSlug string, itemSlug string, renderErr string) {
	project, err := s.store.GetProject(projectSlug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	item, err := s.store.GetWorkItem(projectSlug, itemSlug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	images, _ := s.store.ListWorkItemImages(projectSlug, itemSlug, 30)
	jobs, _ := s.store.ListJobsForWorkItem(projectSlug, itemSlug, 10)
	s.render(w, r, "work-item-detail", PageData{
		Title:       fmt.Sprintf("Work Item: %s", item.Name),
		CurrentPath: "/projects",
		Project:     project,
		WorkItem:    item,
		WorkImages:  images,
		Jobs:        jobs,
		Flash:       r.URL.Query().Get("ok"),
		Error:       renderErr,
	})
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "base", map[string]any{
		"Page":        page,
		"Data":        data,
		"AssetPath":   s.assetPath,
		"RequestPath": r.URL.Path,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) loadManifest(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.logger.Printf("asset manifest unavailable: %v", err)
		return
	}
	manifest := map[string]string{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		s.logger.Printf("failed to parse manifest: %v", err)
		return
	}
	s.manifestMu.Lock()
	s.assetManifest = manifest
	s.manifestMu.Unlock()
}

func (s *Server) assetPath(key string) string {
	s.manifestMu.RLock()
	if hashed, ok := s.assetManifest[key]; ok {
		s.manifestMu.RUnlock()
		return "/static/dist/" + hashed
	}
	s.manifestMu.RUnlock()

	fallback := map[string]string{
		"app.css":             "/static/dist/app.css",
		"app.js":              "/static/dist/app.js",
		"brands.js":           "/static/dist/brands.js",
		"brand-edit.js":       "/static/dist/brand-edit.js",
		"project-detail.js":   "/static/dist/project-detail.js",
		"work-item-detail.js": "/static/dist/work-item-detail.js",
		"job-detail.js":       "/static/dist/job-detail.js",
	}
	if p, ok := fallback[key]; ok {
		return p
	}
	return ""
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) jobWorkerLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		s.processNextJob()
		<-ticker.C
	}
}

func (s *Server) processNextJob() {
	job, err := s.store.ClaimNextQueuedJob()
	if err != nil {
		s.logger.Printf("claim job failed: %v", err)
		return
	}
	if job == nil {
		return
	}

	payload := job.Payload
	if payload.Count < 1 {
		payload.Count = 1
	}
	if payload.Model == "" {
		payload.Model = "both"
	}
	if payload.OutputFormat == "" {
		payload.OutputFormat = "png"
	}
	if payload.ImageSize == "" {
		payload.ImageSize = "1K"
	}

	runPrompt := strings.TrimSpace(job.Prompt)
	if strings.TrimSpace(payload.Adjustment) != "" {
		runPrompt = runPrompt + "\n\nAdjustments:\n" + strings.TrimSpace(payload.Adjustment)
	}

	runSettingsJSON, _ := json.Marshal(payload)
	runID, err := s.store.CreateRun(job.JobID, job.WorkItemID, runPrompt, string(runSettingsJSON))
	if err != nil {
		s.logger.Printf("create run failed for job %d: %v", job.JobID, err)
		_ = s.store.MarkJobFailed(job.JobID, err.Error())
		return
	}

	outputDir := s.store.WorkItemImagesDir(job.ProjectSlug, job.WorkItemSlug, runID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		_ = s.store.MarkRunFailed(runID, err.Error())
		_ = s.store.MarkJobFailed(job.JobID, err.Error())
		return
	}

	args := []string{
		"generate",
		"-prompt", runPrompt,
		"-model", payload.Model,
		"-out", outputDir,
		"-image-size", payload.ImageSize,
		"-n", strconv.Itoa(payload.Count),
		"-output-format", payload.OutputFormat,
	}
	if payload.AspectRatio != "" {
		args = append(args, "-aspect-ratio", payload.AspectRatio)
	}

	var cleanup []func()
	if strings.TrimSpace(job.BrandContent) != "" {
		brandDir, mkErr := os.MkdirTemp("", "imagegen-job-brand-*")
		if mkErr != nil {
			_ = s.store.MarkRunFailed(runID, mkErr.Error())
			_ = s.store.MarkJobFailed(job.JobID, mkErr.Error())
			return
		}
		cleanup = append(cleanup, func() { _ = os.RemoveAll(brandDir) })
		brandFile := filepath.Join(brandDir, "BRAND.md")
		if writeErr := os.WriteFile(brandFile, []byte(job.BrandContent), 0o644); writeErr != nil {
			_ = s.store.MarkRunFailed(runID, writeErr.Error())
			_ = s.store.MarkJobFailed(job.JobID, writeErr.Error())
			for _, fn := range cleanup {
				fn()
			}
			return
		}
		args = append(args, "-brand-dir", brandDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.generatorBinaryPath(), args...)
	cmd.Env = os.Environ()
	output, runErr := cmd.CombinedOutput()
	for _, fn := range cleanup {
		fn()
	}
	if runErr != nil {
		msg := fmt.Sprintf("generate failed: %v\n%s", runErr, strings.TrimSpace(string(output)))
		_ = s.store.MarkRunFailed(runID, msg)
		_ = s.store.MarkJobFailed(job.JobID, msg)
		s.logger.Printf("job %d failed: %v", job.JobID, runErr)
		return
	}

	files, readErr := os.ReadDir(outputDir)
	if readErr != nil {
		_ = s.store.MarkRunFailed(runID, readErr.Error())
		_ = s.store.MarkJobFailed(job.JobID, readErr.Error())
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" && ext != ".ico" {
			continue
		}
		abs := filepath.Join(outputDir, name)
		rel, err := s.store.RelPath(abs)
		if err != nil {
			continue
		}
		_ = s.store.AddRunImage(runID, name, rel, strings.TrimPrefix(ext, "."))
	}

	_ = s.store.MarkRunSucceeded(runID)
	_ = s.store.MarkJobSucceeded(job.JobID)
	s.logger.Printf("job %d succeeded", job.JobID)
}

func (s *Server) generatorBinaryPath() string {
	if _, err := os.Stat("./imagegen"); err == nil {
		return "./imagegen"
	}
	return "imagegen"
}

func loadTemplates(root string) (*template.Template, error) {
	funcs := template.FuncMap{
		"asset": func(resolver any, key string) string {
			if fn, ok := resolver.(func(string) string); ok {
				return fn(key)
			}
			return ""
		},
		"fmtTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.Local().Format("2006-01-02 15:04:05")
		},
		"fmtTimePtr": func(t *time.Time) string {
			if t == nil || t.IsZero() {
				return "-"
			}
			return t.Local().Format("2006-01-02 15:04:05")
		},
	}
	tmpl := template.New("base").Funcs(funcs)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".html" {
			return nil
		}
		_, parseErr := tmpl.ParseFiles(p)
		return parseErr
	})
	if err != nil {
		return nil, err
	}
	if tmpl.Lookup("base") == nil {
		return nil, errors.New("missing base template")
	}
	return tmpl, nil
}

func wantsJSON(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	return strings.Contains(accept, "application/json") ||
		strings.Contains(contentType, "application/json") ||
		r.Header.Get("X-Requested-With") == "fetch"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
