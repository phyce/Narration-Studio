export enum Status {
	Unknown = 0,
	Loading = 1,
	Ready = 2,
	Streaming = 3,
	Generating = 4,
	Error = 5
}

export const StatusDisplayNames = {
	[Status.Unknown]: "⚪ Unknown",
	[Status.Loading]: "🟡 Loading",
	[Status.Ready]: "🟢 Ready",
	[Status.Streaming]: "🟠 Streaming",
	[Status.Generating]: "🔵 Generating",
	[Status.Error]: "🔴 Error"
};