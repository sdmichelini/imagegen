import { ready } from "./core/events";

ready((): void => {
	document.documentElement.classList.add("js-ready");

	for (const form of document.querySelectorAll("form")) {
		form.addEventListener("submit", () => {
			const submit = form.querySelector<HTMLButtonElement>(
				'button[type="submit"][data-loading-text]',
			);
			if (!submit) return;
			if (submit.disabled) return;
			submit.disabled = true;
			submit.setAttribute("aria-busy", "true");
			submit.dataset.originalText = submit.textContent ?? "";
			submit.textContent = submit.dataset.loadingText ?? "Working...";
		});
	}
});
