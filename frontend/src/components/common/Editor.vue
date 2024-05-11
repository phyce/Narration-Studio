<template>
	<div class="overflow-y-auto h-full">
		<vue-monaco-editor
			v-model:value="code"
			language="plaintext"
			theme="vs-dark"
			:options="MONACO_EDITOR_OPTIONS"
			@mount="handleMount"
			@change="handleEditorChange"
		/>
	</div>
</template>

<script lang="ts" setup>
import * as monaco from 'monaco-editor'
import { ref, shallowRef, onUnmounted, watch } from 'vue'

// Accept regexes as a prop
const props = defineProps<{
	regexes: Array<{ regex: RegExp, className: string }>
}>()

const MONACO_EDITOR_OPTIONS = {
	automaticLayout: true,
	formatOnType: true,
	formatOnPaste: true,
}

const code = ref('// some code...')
const editorRef = shallowRef<monaco.editor.IStandaloneCodeEditor | null>(null)
const decorations = ref<string[]>([])

const handleMount = (editor: monaco.editor.IStandaloneCodeEditor) => {
	editorRef.value = editor
	applyRegexHighlighting()
}

const handleEditorChange = () => {
	applyRegexHighlighting()
}

onUnmounted(() => {
	editorRef.value?.deltaDecorations(decorations.value, []); // Use optional chaining
})

watch(props.regexes, () => {
	applyRegexHighlighting()
}, { deep: true })

function applyRegexHighlighting() {
	if (!editorRef.value) return;
	const model = editorRef.value.getModel() ?? undefined; // Use nullish coalescing
	if (!model) return;

	let newDecorations: monaco.editor.IModelDeltaDecoration[] = [];
	props.regexes.forEach(({ regex, className }) => {
		let match;
		while ((match = regex.exec(model.getValue())) !== null) {
			const start = model.getPositionAt(match.index);
			const end = model.getPositionAt(match.index + match[0].length);
			newDecorations.push({
				range: new monaco.Range(start.lineNumber, start.column, end.lineNumber, end.column),
				options: {
					inlineClassName: className
				}
			});
		}
	});
	decorations.value = editorRef.value?.deltaDecorations(decorations.value, newDecorations) ?? decorations.value; // Use optional chaining and nullish coalescing
}
</script>