export enum Status {
	Unknown		= 0,
	Loading		= 1,
	Ready		= 2,
	Streaming	= 3,
	Generating	= 4,
	Playing		= 5,
	Error		= 6,
	Warning     = 7,
}

export const StatusDisplayNames = {
	[Status.Unknown]:		"⚪ Unknown",
	[Status.Loading]:		"<div class=\"icon\">&nbsp;</div>️ Loading",
	[Status.Ready]:			"🟢 Ready",
	[Status.Streaming]:		"🟠 Streaming",
	[Status.Generating]:	"🔵 Generating",
	[Status.Playing]:		"▶️ Playing",
	[Status.Error]:			"🔴 Error",
	[Status.Warning]:       "⚠️ Warning",
};