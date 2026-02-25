import { ready } from "../core/events";

interface GenerateResponse {
	job_id: number;
	status: string;
}

interface JobImage {
	id: number;
	name: string;
	url: string;
}

interface JobStatusResponse {
	id: number;
	status: string;
	error_message?: string;
	images?: JobImage[];
}

const TERMINAL_STATUSES = new Set(["succeeded", "failed"]);

ready((): void => {
	const form = document.getElementById(
		"generate-form",
	) as HTMLFormElement | null;
	const statusEl = document.getElementById(
		"generate-status",
	) as HTMLElement | null;
	if (!form || !statusEl) {
		return;
	}

	form.addEventListener("submit", async (event) => {
		event.preventDefault();

		const submit = form.querySelector<HTMLButtonElement>(
			'button[type="submit"]',
		);
		if (!submit) {
			return;
		}

		submit.disabled = true;
		submit.setAttribute("aria-busy", "true");
		const original = submit.textContent ?? "Queue Generate Job";
		submit.textContent = submit.dataset.loadingText ?? "Queueing...";

		showStatus(statusEl, "Queueing generate job...", false);

		try {
			const body = new URLSearchParams();
			for (const [key, value] of new FormData(form).entries()) {
				if (typeof value === "string") {
					body.append(key, value);
				}
			}

			const res = await fetch(form.action, {
				method: "POST",
				headers: {
					Accept: "application/json",
					"Content-Type": "application/x-www-form-urlencoded",
					"X-Requested-With": "fetch",
				},
				body,
			});

			if (!res.ok) {
				const err = await safeError(res);
				showStatus(statusEl, err, true);
				return;
			}

			const data = (await res.json()) as GenerateResponse;
			showStatus(
				statusEl,
				`Job #${data.job_id} ${data.status}. Polling for completion...`,
				false,
			);
			await pollJob(data.job_id, statusEl);
		} catch (err) {
			const message =
				err instanceof Error ? err.message : "failed to submit job";
			showStatus(statusEl, message, true);
		} finally {
			submit.disabled = false;
			submit.removeAttribute("aria-busy");
			submit.textContent = original;
		}
	});
});

async function pollJob(jobID: number, statusEl: HTMLElement): Promise<void> {
	for (;;) {
		const res = await fetch(`/api/jobs/${jobID}`, {
			headers: { Accept: "application/json" },
		});
		if (!res.ok) {
			showStatus(statusEl, "lost connection while polling job status", true);
			return;
		}

		const data = (await res.json()) as JobStatusResponse;
		if (data.status === "running" || data.status === "queued") {
			showStatus(statusEl, `Job #${jobID} is ${data.status}...`, false);
			await delay(1500);
			continue;
		}

		if (TERMINAL_STATUSES.has(data.status)) {
			if (data.status === "succeeded") {
				showStatus(
					statusEl,
					`Job #${jobID} succeeded. Refreshing images...`,
					false,
				);
				window.location.reload();
				return;
			}
			const errorMsg = data.error_message
				? `Job #${jobID} failed: ${data.error_message}`
				: `Job #${jobID} failed.`;
			showStatus(statusEl, errorMsg, true);
			return;
		}

		await delay(1500);
	}
}

function showStatus(el: HTMLElement, message: string, isError: boolean): void {
	el.classList.remove("hidden", "form-status--error", "form-status--success");
	el.classList.add("form-status");
	el.classList.add(isError ? "form-status--error" : "form-status--success");
	el.textContent = message;
}

async function safeError(res: Response): Promise<string> {
	try {
		const data = (await res.json()) as { error?: string };
		return data.error ?? `request failed (${res.status})`;
	} catch {
		return `request failed (${res.status})`;
	}
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => {
		window.setTimeout(resolve, ms);
	});
}
