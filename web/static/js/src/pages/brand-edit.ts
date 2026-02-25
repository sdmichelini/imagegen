import { byId } from "../core/dom";
import { ready } from "../core/events";

ready((): void => {
	const textarea = byId<HTMLTextAreaElement>("brand-content");
	const preview = byId<HTMLElement>("brand-preview");
	if (!textarea || !preview) return;

	const syncPreview = (): void => {
		preview.textContent = textarea.value;
	};

	syncPreview();
	textarea.addEventListener("input", syncPreview);
});
