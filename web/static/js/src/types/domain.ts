export type JobStatus = "queued" | "running" | "succeeded" | "failed";

export interface JobSummary {
	id: number;
	status: JobStatus;
	projectSlug: string;
	workItemSlug: string;
}
