package webapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var slugSanitizePattern = regexp.MustCompile(`[^a-z0-9]+`)

type Store struct {
	Root   string
	DBPath string
	mu     sync.Mutex
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		Root:   root,
		DBPath: filepath.Join(root, "imagegen.db"),
	}
	if err := s.runMigrations(); err != nil {
		return nil, err
	}
	return s, nil
}

func Slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = slugSanitizePattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func (s *Store) CreateBrand(name string, content string) (Brand, error) {
	slug := Slugify(name)
	if slug == "" {
		return Brand{}, errors.New("brand name is required")
	}
	err := s.execSQL(fmt.Sprintf(`
		INSERT INTO brands (name, slug, content, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s);
	`, q(strings.TrimSpace(name)), q(slug), q(strings.TrimSpace(content)), nowExpr(), nowExpr()))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Brand{}, fmt.Errorf("brand %q already exists", slug)
		}
		return Brand{}, err
	}
	return s.GetBrand(slug)
}

func (s *Store) UpdateBrand(slug string, content string) (Brand, error) {
	slug = Slugify(slug)
	if slug == "" {
		return Brand{}, errors.New("brand slug is required")
	}
	if err := s.execSQL(fmt.Sprintf(`
		UPDATE brands SET content = %s, updated_at = %s WHERE slug = %s;
	`, q(strings.TrimSpace(content)), nowExpr(), q(slug))); err != nil {
		return Brand{}, err
	}
	return s.GetBrand(slug)
}

func (s *Store) GetBrand(slug string) (Brand, error) {
	slug = Slugify(slug)
	rows := []brandRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT id, name, slug, content, created_at, updated_at
		FROM brands
		WHERE slug = %s
		LIMIT 1;
	`, q(slug)), &rows)
	if err != nil {
		return Brand{}, err
	}
	if len(rows) == 0 {
		return Brand{}, os.ErrNotExist
	}
	return rows[0].toBrand(), nil
}

func (s *Store) ListBrands() ([]Brand, error) {
	rows := []brandRow{}
	if err := s.queryJSON(`
		SELECT id, name, slug, content, created_at, updated_at
		FROM brands
		ORDER BY slug ASC;
	`, &rows); err != nil {
		return nil, err
	}
	brands := make([]Brand, 0, len(rows))
	for _, row := range rows {
		brands = append(brands, row.toBrand())
	}
	return brands, nil
}

func (s *Store) CreateProject(name string, defaultBrandSlug string) (Project, error) {
	slug := Slugify(name)
	if slug == "" {
		return Project{}, errors.New("project name is required")
	}
	brandIDExpr := "NULL"
	if b := Slugify(defaultBrandSlug); b != "" {
		brandID, err := s.brandIDBySlug(b)
		if err != nil {
			return Project{}, err
		}
		brandIDExpr = strconv.FormatInt(brandID, 10)
	}
	err := s.execSQL(fmt.Sprintf(`
		INSERT INTO projects (name, slug, default_brand_id, created_at, updated_at)
		VALUES (%s, %s, %s, %s, %s);
	`, q(strings.TrimSpace(name)), q(slug), brandIDExpr, nowExpr(), nowExpr()))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Project{}, fmt.Errorf("project %q already exists", slug)
		}
		return Project{}, err
	}
	return s.GetProject(slug)
}

func (s *Store) GetProject(slug string) (Project, error) {
	slug = Slugify(slug)
	rows := []projectRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT p.id, p.name, p.slug, COALESCE(b.slug, '') AS default_brand_slug,
		       p.created_at, p.updated_at, COUNT(w.id) AS work_item_count
		FROM projects p
		LEFT JOIN brands b ON b.id = p.default_brand_id
		LEFT JOIN work_items w ON w.project_id = p.id
		WHERE p.slug = %s
		GROUP BY p.id, p.name, p.slug, b.slug, p.created_at, p.updated_at
		LIMIT 1;
	`, q(slug)), &rows)
	if err != nil {
		return Project{}, err
	}
	if len(rows) == 0 {
		return Project{}, os.ErrNotExist
	}
	return rows[0].toProject(), nil
}

func (s *Store) ListProjects() ([]Project, error) {
	rows := []projectRow{}
	if err := s.queryJSON(`
		SELECT p.id, p.name, p.slug, COALESCE(b.slug, '') AS default_brand_slug,
		       p.created_at, p.updated_at, COUNT(w.id) AS work_item_count
		FROM projects p
		LEFT JOIN brands b ON b.id = p.default_brand_id
		LEFT JOIN work_items w ON w.project_id = p.id
		GROUP BY p.id, p.name, p.slug, b.slug, p.created_at, p.updated_at
		ORDER BY p.slug ASC;
	`, &rows); err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, row.toProject())
	}
	return projects, nil
}

func (s *Store) CreateWorkItem(projectSlug string, name string, itemType string, prompt string, brandOverrideSlug string) (WorkItem, error) {
	projectSlug = Slugify(projectSlug)
	projectID, err := s.projectIDBySlug(projectSlug)
	if err != nil {
		return WorkItem{}, err
	}
	slug := Slugify(name)
	if slug == "" {
		return WorkItem{}, errors.New("work item name is required")
	}
	p := strings.TrimSpace(prompt)
	if p == "" {
		return WorkItem{}, errors.New("prompt is required")
	}
	t := strings.TrimSpace(itemType)
	if t == "" {
		t = "generic"
	}
	brandIDExpr := "NULL"
	if b := Slugify(brandOverrideSlug); b != "" {
		brandID, err := s.brandIDBySlug(b)
		if err != nil {
			return WorkItem{}, err
		}
		brandIDExpr = strconv.FormatInt(brandID, 10)
	}
	err = s.execSQL(fmt.Sprintf(`
		INSERT INTO work_items (project_id, name, slug, type, prompt, brand_id, created_at, updated_at)
		VALUES (%d, %s, %s, %s, %s, %s, %s, %s);
	`, projectID, q(strings.TrimSpace(name)), q(slug), q(t), q(p), brandIDExpr, nowExpr(), nowExpr()))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return WorkItem{}, fmt.Errorf("work item %q already exists", slug)
		}
		return WorkItem{}, err
	}
	return s.GetWorkItem(projectSlug, slug)
}

func (s *Store) GetWorkItem(projectSlug string, itemSlug string) (WorkItem, error) {
	projectSlug = Slugify(projectSlug)
	itemSlug = Slugify(itemSlug)
	rows := []workItemRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT w.id, w.name, w.slug, w.type, w.prompt, w.project_id,
		       p.slug AS project_slug, COALESCE(b.slug, '') AS brand_override,
		       w.created_at, w.updated_at
		FROM work_items w
		JOIN projects p ON p.id = w.project_id
		LEFT JOIN brands b ON b.id = w.brand_id
		WHERE p.slug = %s AND w.slug = %s
		LIMIT 1;
	`, q(projectSlug), q(itemSlug)), &rows)
	if err != nil {
		return WorkItem{}, err
	}
	if len(rows) == 0 {
		return WorkItem{}, os.ErrNotExist
	}
	return rows[0].toWorkItem(), nil
}

func (s *Store) ListWorkItems(projectSlug string) ([]WorkItem, error) {
	projectSlug = Slugify(projectSlug)
	rows := []workItemRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT w.id, w.name, w.slug, w.type, w.prompt, w.project_id,
		       p.slug AS project_slug, COALESCE(b.slug, '') AS brand_override,
		       w.created_at, w.updated_at
		FROM work_items w
		JOIN projects p ON p.id = w.project_id
		LEFT JOIN brands b ON b.id = w.brand_id
		WHERE p.slug = %s
		ORDER BY w.slug ASC;
	`, q(projectSlug)), &rows)
	if err != nil {
		return nil, err
	}
	items := make([]WorkItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toWorkItem())
	}
	return items, nil
}

func (s *Store) UpdateWorkItemPrompt(projectSlug string, itemSlug string, prompt string) (WorkItem, error) {
	projectSlug = Slugify(projectSlug)
	itemSlug = Slugify(itemSlug)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return WorkItem{}, errors.New("prompt is required")
	}
	err := s.execSQL(fmt.Sprintf(`
		UPDATE work_items
		SET prompt = %s, updated_at = %s
		WHERE id IN (
			SELECT w.id
			FROM work_items w
			JOIN projects p ON p.id = w.project_id
			WHERE p.slug = %s AND w.slug = %s
		);
	`, q(prompt), nowExpr(), q(projectSlug), q(itemSlug)))
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWorkItem(projectSlug, itemSlug)
}

func (s *Store) CreateGenerateJob(projectSlug string, itemSlug string, payload GenerateJobPayload) (Job, error) {
	projectSlug = Slugify(projectSlug)
	itemSlug = Slugify(itemSlug)
	item, err := s.GetWorkItem(projectSlug, itemSlug)
	if err != nil {
		return Job{}, err
	}
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
	raw, _ := json.Marshal(payload)

	rows := []idRow{}
	err = s.queryJSON(fmt.Sprintf(`
		INSERT INTO jobs (work_item_id, status, payload_json, created_at)
		VALUES (%d, 'queued', %s, %s)
		RETURNING id;
	`, item.ID, q(string(raw)), nowExpr()), &rows)
	if err != nil {
		return Job{}, err
	}
	if len(rows) == 0 {
		return Job{}, errors.New("failed to create job")
	}
	return s.GetJob(rows[0].ID)
}

func (s *Store) ListJobs(limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 50
	}
	rows := []jobRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT j.id, j.status, p.slug AS project_slug, p.name AS project_name,
		       w.slug AS work_item_slug, w.name AS work_item_name,
		       j.payload_json, COALESCE(j.error_message, '') AS error_message,
		       j.created_at, j.started_at, j.finished_at, COALESCE(j.run_id, 0) AS run_id
		FROM jobs j
		JOIN work_items w ON w.id = j.work_item_id
		JOIN projects p ON p.id = w.project_id
		ORDER BY j.created_at DESC
		LIMIT %d;
	`, limit), &rows)
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.toJob())
	}
	return jobs, nil
}

func (s *Store) ListJobsForWorkItem(projectSlug string, itemSlug string, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 10
	}
	rows := []jobRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT j.id, j.status, p.slug AS project_slug, p.name AS project_name,
		       w.slug AS work_item_slug, w.name AS work_item_name,
		       j.payload_json, COALESCE(j.error_message, '') AS error_message,
		       j.created_at, j.started_at, j.finished_at, COALESCE(j.run_id, 0) AS run_id
		FROM jobs j
		JOIN work_items w ON w.id = j.work_item_id
		JOIN projects p ON p.id = w.project_id
		WHERE p.slug = %s AND w.slug = %s
		ORDER BY j.created_at DESC
		LIMIT %d;
	`, q(Slugify(projectSlug)), q(Slugify(itemSlug)), limit), &rows)
	if err != nil {
		return nil, err
	}
	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.toJob())
	}
	return jobs, nil
}

func (s *Store) GetJob(jobID int64) (Job, error) {
	rows := []jobRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT j.id, j.status, p.slug AS project_slug, p.name AS project_name,
		       w.slug AS work_item_slug, w.name AS work_item_name,
		       j.payload_json, COALESCE(j.error_message, '') AS error_message,
		       j.created_at, j.started_at, j.finished_at, COALESCE(j.run_id, 0) AS run_id
		FROM jobs j
		JOIN work_items w ON w.id = j.work_item_id
		JOIN projects p ON p.id = w.project_id
		WHERE j.id = %d
		LIMIT 1;
	`, jobID), &rows)
	if err != nil {
		return Job{}, err
	}
	if len(rows) == 0 {
		return Job{}, os.ErrNotExist
	}
	return rows[0].toJob(), nil
}

func (s *Store) ClaimNextQueuedJob() (*JobExecutionContext, error) {
	rows := []claimRow{}
	err := s.queryJSON(`
		SELECT j.id AS job_id, w.id AS work_item_id, p.slug AS project_slug, p.name AS project_name,
		       w.slug AS work_item_slug, w.name AS work_item_name, w.prompt,
		       COALESCE(bw.slug, bp.slug, '') AS brand_slug,
		       COALESCE(bw.content, bp.content, '') AS brand_content,
		       j.payload_json
		FROM jobs j
		JOIN work_items w ON w.id = j.work_item_id
		JOIN projects p ON p.id = w.project_id
		LEFT JOIN brands bw ON bw.id = w.brand_id
		LEFT JOIN brands bp ON bp.id = p.default_brand_id
		WHERE j.status = 'queued'
		ORDER BY j.created_at ASC
		LIMIT 1;
	`, &rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	if err := s.execSQL(fmt.Sprintf(`
		UPDATE jobs SET status = 'running', started_at = %s
		WHERE id = %d AND status = 'queued';
	`, nowExpr(), row.JobID)); err != nil {
		return nil, err
	}

	payload := GenerateJobPayload{}
	if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
		return nil, err
	}
	return &JobExecutionContext{
		JobID:        row.JobID,
		WorkItemID:   row.WorkItemID,
		ProjectSlug:  row.ProjectSlug,
		ProjectName:  row.ProjectName,
		WorkItemSlug: row.WorkItemSlug,
		WorkItemName: row.WorkItemName,
		Prompt:       row.Prompt,
		BrandSlug:    row.BrandSlug,
		BrandContent: row.BrandContent,
		Payload:      payload,
	}, nil
}

func (s *Store) CreateRun(jobID int64, workItemID int64, promptSnapshot string, settingsJSON string) (int64, error) {
	rows := []idRow{}
	err := s.queryJSON(fmt.Sprintf(`
		INSERT INTO runs (job_id, work_item_id, prompt_snapshot, settings_json, status, created_at)
		VALUES (%d, %d, %s, %s, 'running', %s)
		RETURNING id;
	`, jobID, workItemID, q(promptSnapshot), q(settingsJSON), nowExpr()), &rows)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New("failed to create run")
	}
	runID := rows[0].ID
	if err := s.execSQL(fmt.Sprintf(`UPDATE jobs SET run_id = %d WHERE id = %d;`, runID, jobID)); err != nil {
		return 0, err
	}
	return runID, nil
}

func (s *Store) MarkRunSucceeded(runID int64) error {
	return s.execSQL(fmt.Sprintf(`UPDATE runs SET status = 'succeeded', finished_at = %s WHERE id = %d;`, nowExpr(), runID))
}

func (s *Store) MarkRunFailed(runID int64, message string) error {
	return s.execSQL(fmt.Sprintf(`UPDATE runs SET status = 'failed', error_message = %s, finished_at = %s WHERE id = %d;`, q(strings.TrimSpace(message)), nowExpr(), runID))
}

func (s *Store) MarkJobSucceeded(jobID int64) error {
	return s.execSQL(fmt.Sprintf(`UPDATE jobs SET status = 'succeeded', finished_at = %s WHERE id = %d;`, nowExpr(), jobID))
}

func (s *Store) MarkJobFailed(jobID int64, message string) error {
	return s.execSQL(fmt.Sprintf(`UPDATE jobs SET status = 'failed', error_message = %s, finished_at = %s WHERE id = %d;`, q(strings.TrimSpace(message)), nowExpr(), jobID))
}

func (s *Store) AddRunImage(runID int64, filename string, relPath string, format string) error {
	return s.execSQL(fmt.Sprintf(`
		INSERT INTO run_images (run_id, filename, rel_path, format, created_at)
		VALUES (%d, %s, %s, %s, %s);
	`, runID, q(filename), q(relPath), q(format), nowExpr()))
}

func (s *Store) ListWorkItemImages(projectSlug string, itemSlug string, limit int) ([]WorkItemImage, error) {
	if limit <= 0 {
		limit = 40
	}
	rows := []imageRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT ri.id, ri.run_id, ri.filename, ri.created_at
		FROM run_images ri
		JOIN runs r ON r.id = ri.run_id
		JOIN work_items w ON w.id = r.work_item_id
		JOIN projects p ON p.id = w.project_id
		WHERE p.slug = %s AND w.slug = %s
		ORDER BY ri.created_at DESC
		LIMIT %d;
	`, q(Slugify(projectSlug)), q(Slugify(itemSlug)), limit), &rows)
	if err != nil {
		return nil, err
	}
	images := make([]WorkItemImage, 0, len(rows))
	for _, row := range rows {
		images = append(images, row.toImage())
	}
	return images, nil
}

func (s *Store) ListJobImages(jobID int64) ([]WorkItemImage, error) {
	rows := []imageRow{}
	err := s.queryJSON(fmt.Sprintf(`
		SELECT ri.id, ri.run_id, ri.filename, ri.created_at
		FROM run_images ri
		JOIN runs r ON r.id = ri.run_id
		WHERE r.job_id = %d
		ORDER BY ri.created_at ASC;
	`, jobID), &rows)
	if err != nil {
		return nil, err
	}
	images := make([]WorkItemImage, 0, len(rows))
	for _, row := range rows {
		images = append(images, row.toImage())
	}
	return images, nil
}

func (s *Store) ImagePathByID(imageID int64) (string, error) {
	rows := []struct {
		RelPath string `json:"rel_path"`
	}{}
	if err := s.queryJSON(fmt.Sprintf(`SELECT rel_path FROM run_images WHERE id = %d LIMIT 1;`, imageID), &rows); err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", os.ErrNotExist
	}
	return filepath.Join(s.Root, rows[0].RelPath), nil
}

func (s *Store) WorkItemImagesDir(projectSlug string, itemSlug string, runID int64) string {
	rel := filepath.Join("images", Slugify(projectSlug), Slugify(itemSlug), fmt.Sprintf("run-%d", runID))
	return filepath.Join(s.Root, rel)
}

func (s *Store) RelPath(abs string) (string, error) {
	rel, err := filepath.Rel(s.Root, abs)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func (s *Store) runMigrations() error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS brands (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			default_brand_id INTEGER NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(default_brand_id) REFERENCES brands(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS work_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			slug TEXT NOT NULL,
			type TEXT NOT NULL,
			prompt TEXT NOT NULL,
			brand_id INTEGER NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(project_id, slug),
			FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
			FOREIGN KEY(brand_id) REFERENCES brands(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			work_item_id INTEGER NOT NULL,
			run_id INTEGER NULL,
			status TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			error_message TEXT NULL,
			created_at TEXT NOT NULL,
			started_at TEXT NULL,
			finished_at TEXT NULL,
			FOREIGN KEY(work_item_id) REFERENCES work_items(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL UNIQUE,
			work_item_id INTEGER NOT NULL,
			prompt_snapshot TEXT NOT NULL,
			settings_json TEXT NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT NULL,
			created_at TEXT NOT NULL,
			finished_at TEXT NULL,
			FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE,
			FOREIGN KEY(work_item_id) REFERENCES work_items(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS run_images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			filename TEXT NOT NULL,
			rel_path TEXT NOT NULL,
			format TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(run_id) REFERENCES runs(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_status_created ON jobs(status, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_work_item_created ON jobs(work_item_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_work_item_created ON runs(work_item_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_run_images_run_created ON run_images(run_id, created_at);`,
	}
	for _, stmt := range statements {
		if err := s.execSQL(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) projectIDBySlug(slug string) (int64, error) {
	rows := []idRow{}
	if err := s.queryJSON(fmt.Sprintf(`SELECT id FROM projects WHERE slug = %s LIMIT 1;`, q(Slugify(slug))), &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("project %q not found", slug)
	}
	return rows[0].ID, nil
}

func (s *Store) brandIDBySlug(slug string) (int64, error) {
	rows := []idRow{}
	if err := s.queryJSON(fmt.Sprintf(`SELECT id FROM brands WHERE slug = %s LIMIT 1;`, q(Slugify(slug))), &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("brand %q not found", slug)
	}
	return rows[0].ID, nil
}

func (s *Store) execSQL(sqlText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := exec.Command("sqlite3", s.DBPath, sqlText)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sqlite exec failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Store) queryJSON(sqlText string, target any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := exec.Command("sqlite3", "-json", s.DBPath, sqlText)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sqlite query failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	payload := strings.TrimSpace(string(out))
	if payload == "" {
		payload = "[]"
	}
	if err := json.Unmarshal([]byte(payload), target); err != nil {
		return err
	}
	return nil
}

func nowExpr() string {
	return "strftime('%Y-%m-%dT%H:%M:%fZ','now')"
}

func q(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

type idRow struct {
	ID int64 `json:"id"`
}

type brandRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (r brandRow) toBrand() Brand {
	created, _ := time.Parse(time.RFC3339Nano, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	return Brand{ID: r.ID, Name: r.Name, Slug: r.Slug, Content: r.Content, CreatedAt: created, UpdatedAt: updated}
}

type projectRow struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	DefaultBrandSlug string `json:"default_brand_slug"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	WorkItemCount    int    `json:"work_item_count"`
}

func (r projectRow) toProject() Project {
	created, _ := time.Parse(time.RFC3339Nano, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	return Project{ID: r.ID, Name: r.Name, Slug: r.Slug, DefaultBrandSlug: r.DefaultBrandSlug, CreatedAt: created, UpdatedAt: updated, WorkItemCount: r.WorkItemCount}
}

type workItemRow struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Type          string `json:"type"`
	Prompt        string `json:"prompt"`
	ProjectID     int64  `json:"project_id"`
	ProjectSlug   string `json:"project_slug"`
	BrandOverride string `json:"brand_override"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (r workItemRow) toWorkItem() WorkItem {
	created, _ := time.Parse(time.RFC3339Nano, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	return WorkItem{
		ID:            r.ID,
		Name:          r.Name,
		Slug:          r.Slug,
		Type:          r.Type,
		Prompt:        r.Prompt,
		ProjectID:     r.ProjectID,
		ProjectSlug:   r.ProjectSlug,
		BrandOverride: r.BrandOverride,
		CreatedAt:     created,
		UpdatedAt:     updated,
	}
}

type jobRow struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	ProjectSlug  string `json:"project_slug"`
	ProjectName  string `json:"project_name"`
	WorkItemSlug string `json:"work_item_slug"`
	WorkItemName string `json:"work_item_name"`
	PayloadJSON  string `json:"payload_json"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	RunID        int64  `json:"run_id"`
}

func (r jobRow) toJob() Job {
	created, _ := time.Parse(time.RFC3339Nano, r.CreatedAt)
	var startedPtr *time.Time
	if strings.TrimSpace(r.StartedAt) != "" {
		t, _ := time.Parse(time.RFC3339Nano, r.StartedAt)
		startedPtr = &t
	}
	var finishedPtr *time.Time
	if strings.TrimSpace(r.FinishedAt) != "" {
		t, _ := time.Parse(time.RFC3339Nano, r.FinishedAt)
		finishedPtr = &t
	}
	var runID *int64
	if r.RunID > 0 {
		id := r.RunID
		runID = &id
	}
	return Job{
		ID:           r.ID,
		Status:       r.Status,
		ProjectSlug:  r.ProjectSlug,
		ProjectName:  r.ProjectName,
		WorkItemSlug: r.WorkItemSlug,
		WorkItemName: r.WorkItemName,
		PayloadJSON:  r.PayloadJSON,
		ErrorMessage: r.ErrorMessage,
		CreatedAt:    created,
		StartedAt:    startedPtr,
		FinishedAt:   finishedPtr,
		RunID:        runID,
	}
}

type claimRow struct {
	JobID        int64  `json:"job_id"`
	WorkItemID   int64  `json:"work_item_id"`
	ProjectSlug  string `json:"project_slug"`
	ProjectName  string `json:"project_name"`
	WorkItemSlug string `json:"work_item_slug"`
	WorkItemName string `json:"work_item_name"`
	Prompt       string `json:"prompt"`
	BrandSlug    string `json:"brand_slug"`
	BrandContent string `json:"brand_content"`
	PayloadJSON  string `json:"payload_json"`
}

type imageRow struct {
	ID        int64  `json:"id"`
	RunID     int64  `json:"run_id"`
	Filename  string `json:"filename"`
	CreatedAt string `json:"created_at"`
}

func (r imageRow) toImage() WorkItemImage {
	created, _ := time.Parse(time.RFC3339Nano, r.CreatedAt)
	return WorkItemImage{ID: r.ID, RunID: r.RunID, Name: r.Filename, URL: fmt.Sprintf("/images/%d", r.ID), CreatedAt: created}
}
