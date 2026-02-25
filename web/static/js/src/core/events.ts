export function ready(fn: () => void): void {
	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", fn);
		return;
	}
	fn();
}
