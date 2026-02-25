export async function postForm(
	url: string,
	form: HTMLFormElement,
): Promise<Response> {
	const payload = new URLSearchParams();
	for (const [key, value] of new FormData(form).entries()) {
		if (typeof value === "string") {
			payload.append(key, value);
		}
	}
	return fetch(url, {
		method: "POST",
		headers: { "Content-Type": "application/x-www-form-urlencoded" },
		body: payload,
	});
}
