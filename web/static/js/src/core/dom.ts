export function byId<T extends HTMLElement = HTMLElement>(
	id: string,
): T | null {
	return document.getElementById(id) as T | null;
}

export function on<K extends keyof DocumentEventMap>(
	event: K,
	selector: string,
	handler: (event: DocumentEventMap[K], target: HTMLElement) => void,
): void {
	document.addEventListener(event, (rawEvent) => {
		const source = rawEvent.target;
		if (!(source instanceof Element)) {
			return;
		}
		const target = source.closest(selector);
		if (!(target instanceof HTMLElement)) {
			return;
		}
		handler(rawEvent as DocumentEventMap[K], target);
	});
}
