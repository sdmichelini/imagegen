package webapp

import "time"

type Brand struct {
	ID        int64
	Name      string
	Slug      string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Project struct {
	ID               int64
	Name             string
	Slug             string
	DefaultBrandSlug string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	WorkItemCount    int
}

type WorkItem struct {
	ID            int64
	Name          string
	Slug          string
	Type          string
	Prompt        string
	ProjectID     int64
	ProjectSlug   string
	BrandOverride string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type WorkItemImage struct {
	ID        int64
	RunID     int64
	Name      string
	URL       string
	CreatedAt time.Time
}

type GenerateJobPayload struct {
	Model        string `json:"model"`
	Count        int    `json:"count"`
	OutputFormat string `json:"output_format"`
	ImageSize    string `json:"image_size"`
	AspectRatio  string `json:"aspect_ratio"`
	Adjustment   string `json:"adjustment"`
}

type Job struct {
	ID           int64
	Status       string
	ProjectSlug  string
	ProjectName  string
	WorkItemSlug string
	WorkItemName string
	PayloadJSON  string
	ErrorMessage string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	RunID        *int64
}

type JobExecutionContext struct {
	JobID        int64
	WorkItemID   int64
	ProjectSlug  string
	ProjectName  string
	WorkItemSlug string
	WorkItemName string
	Prompt       string
	BrandSlug    string
	BrandContent string
	Payload      GenerateJobPayload
}
