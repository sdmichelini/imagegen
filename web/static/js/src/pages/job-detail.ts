import { ready } from "../core/events";

interface JobStatusResponse {
	id: number;
	status: string;
	error_message?: string;
}

const TERMINAL_STATUSES = new Set(["succeeded", "failed"]);

ready((): void => {
	const root = document.getElementById("job-detail-root") as HTMLElement | null;
	if (!root) return;

	const jobID = Number(root.dataset.jobId ?? "0");
	const status = root.dataset.jobStatus ?? "";
	if (!Number.isFinite(jobID) || jobID < 1) return;
	if (TERMINAL_STATUSES.has(status)) return;

	const badge = document.getElementById(
		"job-status-badge",
	) as HTMLElement | null;
	const pollNotice = document.getElementById(
		"job-poll-notice",
	) as HTMLElement | null;

	void pollJob(jobID, status, badge, pollNotice);
});

async function pollJob(
	jobID: number,
	initialStatus: string,
	badge: HTMLElement | null,
	pollNotice: HTMLElement | null,
): Promise<void> {
	const currentStatus = initialStatus;
	if (pollNotice) {
		pollNotice.textContent = `Polling job #${jobID}...`;
		pollNotice.classList.remove("hidden");
	}

	for (;;) {
		const res = await fetch(`/api/jobs/${jobID}`, {
			headers: { Accept: "application/json" },
		});
		if (!res.ok) {
			if (pollNotice) {
				pollNotice.textContent = "Unable to poll job status right now.";
			}
			return;
		}

		const data = (await res.json()) as JobStatusResponse;
		if (data.status !== currentStatus) {
			window.location.reload();
			return;
		}

		if (badge) {
			badge.textContent = data.status;
			badge.className = `status-badge status-badge--${data.status}`;
		}

		if (TERMINAL_STATUSES.has(data.status)) {
			window.location.reload();
			return;
		}

		await delay(1500);
	}
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => {
		window.setTimeout(resolve, ms);
	});
}
