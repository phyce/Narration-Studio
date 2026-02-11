<script setup lang="ts">
import '../../css/pages/server.css';

import Button from 'primevue/button';
import Checkbox from 'primevue/checkbox';
import Dropdown from 'primevue/dropdown';
import InputGroup from 'primevue/inputgroup';
import InputNumber from 'primevue/inputnumber';
import InputText from 'primevue/inputtext';
import Panel from 'primevue/panel';
import {computed, nextTick, onBeforeMount, onBeforeUnmount, ref, watch} from 'vue';
import {
	GenerateServerCommand,
	GetServerLogs,
	GetServerStatus,
	SelectFile,
	StartDaemonServer,
	StopDaemonServer
} from '../../../wailsjs/go/main/App';
import {eventManager} from '../../util/eventManager';
import {ServerStatus} from "../interfaces/server";

const serverModes = [
	{label: 'HTTP Server', value: 'http'},
	// {label: 'WebSocket Server', value: 'websocket'},
	// {label: 'gRPC Server', value: 'grpc'},
	// {label: 'TCP Server', value: 'tcp'},
	// {label: 'Named Pipe Server', value: 'namedpipe'},
	// {label: 'File System Server', value: 'filesystem'}
];

const selectedMode = ref<string>('http');
const port = ref<number>(8989);
const host = ref<string>('localhost');
const configFile = ref<string>('');
const useCustomConfig = ref<boolean>(false);
const serverStatus = ref<ServerStatus>({
	running: false,
	output: '',
	error: '',
	pid: 0,
	version: '',
	uptime: '',
	processedMessages: 0
});
const loading = ref<boolean>(false);
const generatedCommand = ref<string>('');
const statusCheckInterval = ref<number | null>(null);
const serverLogs = ref<string>('');
const logsCollapsed = ref<boolean>(true);
const logsMetadata = ref<{ totalSize?: number; lineCount?: number; showing?: number }>({});
const logsRefreshInterval = ref<number | null>(null);
const logsContainerRef = ref<HTMLElement | null>(null);
const logsPanelRef = ref<HTMLElement | null>(null);
const autoScrollEnabled = ref<boolean>(true);

const isServerRunning = computed(() => serverStatus.value.running);
const commandDisplay = computed(() => {
	if (isServerRunning.value) {
		return `http://${host.value}:${port.value}`;
	}
	return generatedCommand.value;
});

async function fetchLogs() {
	try {
		const result = await GetServerLogs();
		const logsData = JSON.parse(result);
		serverLogs.value = logsData.logs || '';
		logsMetadata.value = {
			totalSize: logsData.totalSize,
			lineCount: logsData.lineCount,
			showing: logsData.showing
		};

		if (autoScrollEnabled.value) {
			await nextTick();
			scrollToBottom();
		}
	} catch (error) {
		console.error('Failed to fetch logs:', error);
	}
}

function scrollToBottom() {
	if (logsContainerRef.value) {
		logsContainerRef.value.scrollTop = logsContainerRef.value.scrollHeight;
	}
}

function handleLogsScroll() {
	if (!logsContainerRef.value) return;

	const { scrollTop, scrollHeight, clientHeight } = logsContainerRef.value;
	const isAtBottom = scrollHeight - scrollTop - clientHeight < 10;

	autoScrollEnabled.value = isAtBottom;
}

async function startServer() {
	loading.value = true;
	try {
		const result = await StartDaemonServer(
			selectedMode.value,
			port.value,
			host.value,
			configFile.value
		);
		serverStatus.value = JSON.parse(result);
		startStatusPolling();
	} catch (error) {
		console.error('Failed to start server:', error);
	} finally {
		loading.value = false;
	}
}

async function stopServer() {
	loading.value = true;
	try {
		const result = await StopDaemonServer();
		serverStatus.value = JSON.parse(result);
		stopStatusPolling();
	} catch (error) {
		console.error('Failed to stop server:', error);
	} finally {
		loading.value = false;
	}
}

async function toggleServer() {
	if (isServerRunning.value) {
		await stopServer();
	} else {
		await startServer();
	}
}

async function refreshStatus() {
	try {
		const result = await GetServerStatus();
		serverStatus.value = JSON.parse(result);
	} catch (error) {
		console.error('Failed to get server status:', error);
	}
}

async function updateGeneratedCommand() {
	try {
		generatedCommand.value = await GenerateServerCommand(
			selectedMode.value,
			port.value,
			host.value,
			configFile.value
		);
	} catch (error) {
		console.error('Failed to generate command:', error);
	}
}

async function selectConfigFile() {
	const selected = await SelectFile(configFile.value);
	if (selected) {
		configFile.value = selected;
		await updateGeneratedCommand();
	}
}

async function copyCommandToClipboard() {
	try {
		await navigator.clipboard.writeText(commandDisplay.value);
		eventManager.emit('notification.send', {
			severity: 'success',
			summary: 'Copied',
			detail: 'Command copied to clipboard',
			life: 2000
		});
	} catch (error) {
		eventManager.emit('notification.send', {
			severity: 'error',
			summary: 'Copy Failed',
			detail: 'Failed to copy command to clipboard',
			life: 3000
		});
	}
}

function startStatusPolling() {
	if (statusCheckInterval.value === null) {
		statusCheckInterval.value = window.setInterval(refreshStatus, 5000);
	}
}

function stopStatusPolling() {
	if (statusCheckInterval.value !== null) {
		clearInterval(statusCheckInterval.value);
		statusCheckInterval.value = null;
	}
}

function startLogsRefresh() {
	if (logsRefreshInterval.value === null && !logsCollapsed.value) {
		autoScrollEnabled.value = true;
		fetchLogs();
		logsRefreshInterval.value = window.setInterval(fetchLogs, 3000);
	}
}

function stopLogsRefresh() {
	if (logsRefreshInterval.value !== null) {
		clearInterval(logsRefreshInterval.value);
		logsRefreshInterval.value = null;
	}
}

watch(logsCollapsed, async (isCollapsed) => {
	if (isCollapsed) {
		stopLogsRefresh();
	} else if (isServerRunning.value) {
		startLogsRefresh();
		await nextTick();
		if (logsPanelRef.value) {
			logsPanelRef.value.scrollIntoView({ behavior: 'smooth', block: 'start' });
		}
	}
});

watch(isServerRunning, (running) => {
	if (!running) {
		stopLogsRefresh();
	}
});

onBeforeMount(async () => {
	await refreshStatus();
	await updateGeneratedCommand();

	if (serverStatus.value.running) {
		startStatusPolling();
	}
});

onBeforeUnmount(() => {
	stopStatusPolling();
	stopLogsRefresh();
});
</script>

<template>
	<div class="server">
		<div class="server__header">
			<Button
				:class="isServerRunning ? 'button-stop' : 'button-start'"
				@click="toggleServer"
				:disabled="loading"
				:title="isServerRunning ? 'Stop Server' : 'Start Server'"
				:aria-label="isServerRunning ? 'Stop Server' : 'Start Server'"
			>
				<i :class="isServerRunning ? 'pi pi-stop' : 'pi pi-play'"/>&nbsp;{{ isServerRunning ? 'Stop' : 'Start' }}
			</Button>

			<Dropdown
				id="mode"
				v-model="selectedMode"
				:options="serverModes"
				optionLabel="label"
				optionValue="value"
				placeholder="Select server mode"
				disabled
				@change="updateGeneratedCommand"
				title="Server Mode"
				class="server__header__mode"
			/>

			<InputText
				id="host"
				v-model="host"
				placeholder="localhost"
				@input="updateGeneratedCommand"
				:disabled="isServerRunning"
				class="server__header__host"
				title="Host"
			/>

			<InputNumber
				id="port"
				v-model="port"
				:min="1"
				:max="65535"
				:useGrouping="false"
				placeholder="8989"
				@input="updateGeneratedCommand"
				:disabled="isServerRunning"
				class="server__header__port"
				title="Port"
			/>

			<div class="flex items-center gap-2 server__header__custom-config-wrapper">
				<Checkbox
					v-model="useCustomConfig"
					inputId="useCustomConfig"
					:binary="true"
					:disabled="isServerRunning"
					class="server__header__custom-config"
				/>
				<label for="useCustomConfig" class="server__header__custom-config-label">
					Custom config
				</label>
			</div>
		</div>

		<div v-if="useCustomConfig" class="server__config-row">
			<InputGroup>
				<InputText
					id="config"
					v-model="configFile"
					placeholder="Path to config file"
					@input="updateGeneratedCommand"
					:title="configFile"
					:disabled="isServerRunning"
				/>
				<Button
					icon="pi pi-folder-open"
					@click="selectConfigFile"
					title="Browse"
					aria-label="Browse for config file"
					:disabled="isServerRunning"
				/>
			</InputGroup>
		</div>

		<div class="server__command-row">
			<InputText
				id="command"
				:value="commandDisplay"
				readonly
				@click="copyCommandToClipboard"
				class="w-full font-mono text-sm cursor-pointer"
				title="Click to copy"
			/>
		</div>

		<div v-if="isServerRunning" class="server__status-row">
			<div class="server__status-item">
				<span class="server__status-label">Status:</span>
				<span class="server__status-value">
					{{ isServerRunning ? 'Running' : 'Stopped' }}
				</span>
			</div>
			<div class="server__status-item">
				<span class="server__status-label">Uptime:</span>
				<span class="server__status-value">{{ serverStatus.uptime }}</span>
			</div>
			<div class="server__status-item">
				<span class="server__status-label">Processed Messages:</span>
				<span class="server__status-value">{{ serverStatus.processedMessages }}</span>
			</div>
			<div class="server__status-item">
				<span class="server__status-label">PID:</span>
				<span class="server__status-value">{{ serverStatus.pid }}</span>
			</div>
		</div>

		<div v-if="isServerRunning" class="server__logs-row">
			<div ref="logsPanelRef">
				<Panel v-model:collapsed="logsCollapsed" toggleable>
					<template #header>
						<div class="flex items-center gap-2">
							<i class="pi pi-file-edit"></i>
							<span>Server Logs</span>
							<small v-if="logsMetadata.showing" class="text-gray-400 ml-2">
								(showing {{ logsMetadata.showing }} of {{ logsMetadata.lineCount }} lines)
							</small>
						</div>
					</template>
					<div class="server__logs-content">
						<pre
							v-if="serverLogs"
							ref="logsContainerRef"
							@scroll="handleLogsScroll"
							class="server__logs-text"
						>{{ serverLogs }}</pre>
						<p v-else class="text-gray-400">Loading logs...</p>
					</div>
				</Panel>
			</div>
		</div>

	</div>
</template>

<style scoped>

</style>