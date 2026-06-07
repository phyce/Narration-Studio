<script setup lang="ts">
import {computed, onMounted, onUnmounted, reactive, ref} from "vue";
import {eventManager} from "../util/eventManager";
import {GetActivityLog, GetActivityLogOlder} from "../../bindings/nstudio/app";

interface Entry {
	id: number;
	chainId: number;
	timestamp: string;
	severity: string;
	summary: string;
	detail?: string;
	statusId: number;
	statusLabel: string;
	source?: string;
}

interface Chain {
	chainId: number;
	frames: Entry[];
	representative: Entry;
}

const severities = ["error", "warning", "success", "info", "status"];
const olderPageSize = 200;

const entries = ref<Entry[]>([]);
const severityFilters = reactive<Record<string, boolean>>({
	error: true,
	warning: true,
	success: true,
	info: true,
	status: true,
});
const statusFilter = ref<string>("");
const searchText = ref<string>("");
const expandedChains = reactive<Set<number>>(new Set());
const hasMore = ref<boolean>(true);
const loadingOlder = ref<boolean>(false);

function mergeEntries(incoming: Entry[]) {
	const byID = new Map<number, Entry>();
	for (const entry of entries.value) {
		byID.set(entry.id, entry);
	}
	for (const entry of incoming) {
		byID.set(entry.id, entry);
	}
	entries.value = Array.from(byID.values()).sort((a, b) => a.id - b.id);
}

function onAppended(entry: Entry) {
	if (!entry || typeof entry.id !== "number") {
		return;
	}
	mergeEntries([entry]);
}

let unsubscribe = () => {
};

onMounted(async () => {
	unsubscribe = eventManager.subscribe("activity.appended", onAppended);
	const json = await GetActivityLog();
	try {
		const recent = JSON.parse(json) as Entry[];
		mergeEntries(recent ?? []);
	} catch (error) {
		console.error("Failed to parse activity log", error);
	}
});

onUnmounted(() => {
	unsubscribe();
});

async function loadOlder() {
	if (loadingOlder.value || !hasMore.value || entries.value.length === 0) {
		return;
	}
	loadingOlder.value = true;
	try {
		const smallestID = entries.value[0].id;
		const json = await GetActivityLogOlder(smallestID, olderPageSize);
		const older = (JSON.parse(json) as Entry[]) ?? [];
		if (older.length === 0) {
			hasMore.value = false;
		} else {
			mergeEntries(older);
		}
	} catch (error) {
		console.error("Failed to load older activity", error);
	} finally {
		loadingOlder.value = false;
	}
}

function onScroll(event: Event) {
	const target = event.target as HTMLElement;
	if (target.scrollHeight - target.scrollTop - target.clientHeight < 120) {
		loadOlder();
	}
}

const distinctStatusLabels = computed(() => {
	const labels = new Set<string>();
	for (const entry of entries.value) {
		if (entry.statusLabel) {
			labels.add(entry.statusLabel);
		}
	}
	return Array.from(labels).sort();
});

const filteredEntries = computed(() => {
	const search = searchText.value.trim().toLowerCase();
	return entries.value.filter((entry) => {
		if (!severityFilters[entry.severity]) {
			return false;
		}
		if (statusFilter.value && entry.statusLabel !== statusFilter.value) {
			return false;
		}
		if (search) {
			const haystack = `${entry.summary} ${entry.detail ?? ""} ${entry.source ?? ""}`.toLowerCase();
			if (!haystack.includes(search)) {
				return false;
			}
		}
		return true;
	});
});

const chains = computed<Chain[]>(() => {
	const groups = new Map<number, Entry[]>();
	for (const entry of filteredEntries.value) {
		const frames = groups.get(entry.chainId);
		if (frames) {
			frames.push(entry);
		} else {
			groups.set(entry.chainId, [entry]);
		}
	}
	const result: Chain[] = [];
	for (const frames of groups.values()) {
		frames.sort((a, b) => a.id - b.id);
		result.push({
			chainId: frames[0].chainId,
			frames,
			representative: frames[frames.length - 1],
		});
	}
	result.sort((a, b) => b.representative.id - a.representative.id);
	return result;
});

function toggleChain(chainId: number) {
	if (expandedChains.has(chainId)) {
		expandedChains.delete(chainId);
	} else {
		expandedChains.add(chainId);
	}
}

function isExpandable(chain: Chain): boolean {
	return chain.frames.length > 1 || !!chain.representative.detail;
}

// Outermost (user-facing) frame first, drilling down into causes.
function orderedFrames(chain: Chain): Entry[] {
	return chain.frames.slice().reverse();
}

function formatTime(timestamp: string): string {
	const date = new Date(timestamp);
	if (isNaN(date.getTime())) {
		return timestamp;
	}
	return date.toLocaleString();
}
</script>

<template>
	<div class="activity-log">
		<div class="toolbar">
			<div class="severities">
				<label v-for="severity in severities" :key="severity" class="severity-toggle">
					<input type="checkbox" v-model="severityFilters[severity]"/>
					<span :class="['badge', severity]">{{ severity }}</span>
				</label>
			</div>
			<select v-model="statusFilter" class="status-select">
				<option value="">All statuses</option>
				<option v-for="label in distinctStatusLabels" :key="label" :value="label">{{ label }}</option>
			</select>
			<input v-model="searchText" class="search" type="text" placeholder="Search…"/>
		</div>

		<div class="entries" @scroll="onScroll">
			<div
				v-for="chain in chains"
				:key="chain.chainId"
				:class="['entry', chain.representative.severity, {expandable: isExpandable(chain)}]"
				@click="isExpandable(chain) && toggleChain(chain.chainId)"
			>
				<div class="entry-row">
					<span class="chevron">{{ isExpandable(chain) ? (expandedChains.has(chain.chainId) ? "▾" : "▸") : "" }}</span>
					<span :class="['dot', chain.representative.severity]"></span>
					<span class="time">{{ formatTime(chain.representative.timestamp) }}</span>
					<span class="summary">{{ chain.representative.summary }}</span>
					<span v-if="chain.representative.statusLabel" class="status-tag">{{ chain.representative.statusLabel }}</span>
					<span v-if="chain.frames.length > 1" class="chain-count">⛓ {{ chain.frames.length }}</span>
					<span v-if="chain.representative.source" class="source">{{ chain.representative.source }}</span>
				</div>

				<div v-if="expandedChains.has(chain.chainId)" class="frames">
					<div
						v-for="(frame, index) in orderedFrames(chain)"
						:key="frame.id"
						class="frame"
						:style="{ marginLeft: index * 16 + 'px' }"
					>
						<div class="frame-head">
							<span v-if="index > 0" class="cause-arrow">↳ caused by</span>
							<span class="frame-summary">{{ frame.summary }}</span>
							<span v-if="frame.source" class="frame-source">{{ frame.source }}</span>
						</div>
						<div v-if="frame.detail" class="frame-detail">{{ frame.detail }}</div>
					</div>
				</div>
			</div>

			<div v-if="chains.length === 0" class="empty">No activity to show.</div>
			<div v-if="hasMore && entries.length > 0" class="load-older" @click="loadOlder">
				{{ loadingOlder ? "Loading…" : "Load older entries" }}
			</div>
		</div>
	</div>
</template>

<style scoped>
.activity-log {
	display: flex;
	flex-direction: column;
	height: 100vh;
	font-size: 13px;
}

.toolbar {
	display: flex;
	align-items: center;
	gap: 12px;
	padding: 8px 12px;
	background-color: #1e2a38;
	border-bottom: 1px solid #2f3e50;
	flex-wrap: wrap;
}

.severities {
	display: flex;
	gap: 8px;
	flex-wrap: wrap;
}

.severity-toggle {
	display: flex;
	align-items: center;
	gap: 4px;
	cursor: pointer;
}

.badge {
	text-transform: capitalize;
}

.status-select,
.search {
	background-color: #16212d;
	color: white;
	border: 1px solid #2f3e50;
	border-radius: 4px;
	padding: 4px 8px;
}

.search {
	flex: 1;
	min-width: 120px;
}

.entries {
	flex: 1;
	overflow-y: auto;
	padding: 4px 0;
}

.entry {
	padding: 6px 12px;
	border-bottom: 1px solid #232f3d;
	border-left: 3px solid transparent;
}

.entry.expandable {
	cursor: pointer;
}

.entry.error {
	border-left-color: #ef4444;
}

.entry.warning {
	border-left-color: #f59e0b;
}

.entry.success {
	border-left-color: #22c55e;
}

.entry.info {
	border-left-color: #3b82f6;
}

.entry.status {
	border-left-color: #64748b;
}

.entry-row {
	display: flex;
	align-items: center;
	gap: 10px;
}

.chevron {
	width: 12px;
	color: #93a4b8;
	flex-shrink: 0;
}

.dot {
	width: 8px;
	height: 8px;
	border-radius: 50%;
	flex-shrink: 0;
}

.dot.error {
	background-color: #ef4444;
}

.dot.warning {
	background-color: #f59e0b;
}

.dot.success {
	background-color: #22c55e;
}

.dot.info {
	background-color: #3b82f6;
}

.dot.status {
	background-color: #64748b;
}

.time {
	color: #93a4b8;
	white-space: nowrap;
	font-variant-numeric: tabular-nums;
}

.summary {
	flex: 1;
	text-align: left;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.status-tag {
	color: #93a4b8;
	background-color: #16212d;
	border-radius: 4px;
	padding: 1px 6px;
	white-space: nowrap;
}

.chain-count {
	color: #cbd5e1;
	white-space: nowrap;
}

.source {
	color: #6b7f96;
	white-space: nowrap;
}

.frames {
	margin-top: 6px;
	padding-left: 18px;
	display: flex;
	flex-direction: column;
	gap: 4px;
}

.frame {
	text-align: left;
	border-left: 2px solid #2f3e50;
	padding-left: 8px;
}

.frame-head {
	display: flex;
	align-items: baseline;
	gap: 6px;
	flex-wrap: wrap;
}

.cause-arrow {
	color: #6b7f96;
	white-space: nowrap;
}

.frame-summary {
	color: #d7e0ea;
}

.frame-source {
	color: #6b7f96;
}

.frame-detail {
	color: #93a4b8;
	white-space: pre-wrap;
	margin-top: 2px;
}

.empty {
	color: #6b7f96;
	padding: 24px;
	text-align: center;
}

.load-older {
	color: #93a4b8;
	padding: 12px;
	text-align: center;
	cursor: pointer;
}

.load-older:hover {
	color: white;
}
</style>
