export interface ServerStatus {
	running: boolean;
	output: string;
	error: string;
	pid: number;
	version: string;
	uptime: string;
	processedMessages: number;
}