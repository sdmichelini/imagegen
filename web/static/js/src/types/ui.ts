export type AsyncState =
	| { status: "idle" }
	| { status: "loading" }
	| { status: "success"; message: string }
	| { status: "error"; message: string };

export type Nullable<T> = T | null;
