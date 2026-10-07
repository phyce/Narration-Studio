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
	GetConfigFilePath,
	GetServerLogs,
	GetServerStatus,
	SelectFile,
	StartDaemonServer,
	StopDaemonServer
} from '../../../bindings/nstudio/app';
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
const appConfigFile = ref<string>('');
const customConfigFile = ref<string>('');
const useCustomConfig = ref<boolean>(false);
// The field shows the app's own config file until "Use custom config file" is ticked.
const configFile = computed({
	get: () => useCustomConfig.value ? customConfigFile.value : appConfigFile.value,
	set: (value: string) => { customConfigFile.value = value; },
});
const serverConfigFile = computed(() => useCustomConfig.value ? customConfigFile.value : '');
const serverStatus = ref<ServerStatus>({
	running: false,
	output: '',
	error: '',
	pid: 0,
	version: '',
	uptime: '',
	processedMessages: 0
});
const starting = ref<boolean>(false);
const stopping = ref<boolean>(false);
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
const settingsLocked = computed(() => isServerRunning.value || starting.value);
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

// Starting waits for the server to load its models, which can take minutes; Stop cancels it.
async function startServer() {
	starting.value = true;
	try {
		const result = await StartDaemonServer(
			selectedMode.value,
			port.value,
			host.value,
			serverConfigFile.value
		);
		serverStatus.value = JSON.parse(result);
		if (serverStatus.value.running) {
			startStatusPolling();
		}
	} catch (error) {
		console.error('Failed to start server:', error);
	} finally {
		starting.value = false;
	}
}

async function stopServer() {
	stopping.value = true;
	try {
		const result = await StopDaemonServer();
		serverStatus.value = JSON.parse(result);
		stopStatusPolling();
	} catch (error) {
		console.error('Failed to stop server:', error);
	} finally {
		stopping.value = false;
	}
}

async function toggleServer() {
	if (isServerRunning.value || starting.value) {
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
			serverConfigFile.value
		);
	} catch (error) {
		console.error('Failed to generate command:', error);
	}
}

async function selectConfigFile() {
	const selected = await SelectFile(configFile.value);
	if (selected) {
		customConfigFile.value = selected;
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

watch(useCustomConfig, updateGeneratedCommand);

onBeforeMount(async () => {
	appConfigFile.value = await GetConfigFilePath();
	customConfigFile.value = appConfigFile.value;
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
	<div class="page page--column">
		<div class="page-toolbar">
			<Button
				:icon="settingsLocked ? 'pi pi-stop' : 'pi pi-play'"
				:label="settingsLocked ? 'Stop server' : 'Start server'"
				:severity="settingsLocked ? 'danger' : 'success'"
				:loading="stopping"
				:disabled="stopping"
				@click="toggleServer"
			/>
			<span v-if="starting" class="server__state server__state--starting">
				<i class="pi pi-spin pi-spinner"/>
				Starting… loading the voice models might take a while
			</span>
			<span v-else class="server__state" :class="{'server__state--running': isServerRunning}">
				<i class="pi pi-circle-fill"/>
				{{ isServerRunning ? 'Running' : 'Stopped' }}
			</span>
			<template v-if="isServerRunning && !starting">
				<div class="page-toolbar__spacer"/>
				<div class="server__stats">
					<div class="server__stat">
						<span class="field__label">Uptime</span>
						<span class="server__stat__value">{{ serverStatus.uptime || '—' }}</span>
					</div>
					<div class="server__stat">
						<span class="field__label">Messages processed</span>
						<span class="server__stat__value">{{ serverStatus.processedMessages }}</span>
					</div>
					<div class="server__stat">
						<span class="field__label">Process ID</span>
						<span class="server__stat__value">{{ serverStatus.pid }}</span>
					</div>
				</div>
			</template>
		</div>

		<div class="page-body server__body">
			<div class="card field">
				<label class="field__label" for="server-command">
					{{ isServerRunning ? 'Server address' : 'Command line' }}
				</label>
				<InputGroup>
					<InputText
						id="server-command"
						:value="commandDisplay"
						readonly
						class="server__command"
						:title="commandDisplay"
					/>
					<Button
						icon="pi pi-copy"
						title="Copy"
						aria-label="Copy"
						@click="copyCommandToClipboard"
					/>
				</InputGroup>
			</div>

			<div class="card server__settings">
				<div class="server__fields">
					<div class="field">
						<label class="field__label" for="server-mode">Mode</label>
						<Dropdown
							inputId="server-mode"
							v-model="selectedMode"
							:options="serverModes"
							optionLabel="label"
							optionValue="value"
							disabled
							@change="updateGeneratedCommand"
						/>
					</div>
					<div class="field">
						<label class="field__label" for="server-host">Host</label>
						<InputText
							id="server-host"
							v-model="host"
							placeholder="localhost"
							@input="updateGeneratedCommand"
							:disabled="settingsLocked"
						/>
					</div>
					<div class="field">
						<label class="field__label" for="server-port">Port</label>
						<InputNumber
							inputId="server-port"
							v-model="port"
							:min="1"
							:max="65535"
							:useGrouping="false"
							placeholder="8989"
							@input="updateGeneratedCommand"
							:disabled="settingsLocked"
							class="w-full"
							inputClass="w-full min-w-0"
						/>
					</div>
				</div>

				<InputGroup>
					<InputText
						v-model="configFile"
						placeholder="Path to a config file"
						aria-label="Config file"
						@input="updateGeneratedCommand"
						:title="configFile"
						:disabled="!useCustomConfig || settingsLocked"
					/>
					<Button
						icon="pi pi-folder-open"
						@click="selectConfigFile"
						title="Browse"
						aria-label="Browse for a config file"
						:disabled="!useCustomConfig || settingsLocked"
					/>
				</InputGroup>
				<div class="check-row">
					<Checkbox
						v-model="useCustomConfig"
						inputId="server-custom-config"
						:binary="true"
						:disabled="settingsLocked"
					/>
					<label for="server-custom-config">Use custom config file</label>
				</div>
			</div>

			<div v-if="isServerRunning" ref="logsPanelRef">
				<Panel v-model:collapsed="logsCollapsed" toggleable>
					<template #header>
						<div class="server__logs-header">
							<i class="pi pi-align-left"></i>
							<span>Server logs</span>
							<small v-if="logsMetadata.showing" class="text-secondary">
								showing {{ logsMetadata.showing }} of {{ logsMetadata.lineCount }} lines
							</small>
						</div>
					</template>
					<pre
						v-if="serverLogs"
						ref="logsContainerRef"
						@scroll="handleLogsScroll"
						class="server__logs-text"
					>{{ serverLogs }}</pre>
					<p v-else class="text-secondary m-0">Loading logs…</p>
				</Panel>
			</div>
		</div>
	</div>
</template>
