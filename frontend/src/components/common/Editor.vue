<template>
	<div class="overflow-y-auto h-full">
		<vue-monaco-editor
			v-model:value="internalText"
			language="plaintext"
			theme="vs-dark"
			:options="MONACO_EDITOR_OPTIONS"
			@mount="handleMount"
			@change="handleEditorChange"
		/>
	</div>
</template>

<script lang="ts" setup>
import * as monaco from 'monaco-editor';
import { ref, shallowRef, onUnmounted, watch, defineProps, defineEmits } from 'vue';

const props = defineProps<{
	regexes: Array<{ regex: RegExp, className: string }>,
	text: string
}>();

const emit = defineEmits(['update:text']);

const MONACO_EDITOR_OPTIONS = {
	automaticLayout: true,
	formatOnType: true,
	formatOnPaste: true,
};

const internalText = ref(props.text);
const editorRef = shallowRef<monaco.editor.IStandaloneCodeEditor | null>(null);
const decorations = ref<string[]>([]);

const handleMount = (editor: monaco.editor.IStandaloneCodeEditor) => {
	editorRef.value = editor;
	applyRegexHighlighting();
};

const handleEditorChange = () => {
	if (editorRef.value) {
		const value = editorRef.value.getValue();
		internalText.value = value;
		emit('update:text', value);
		applyRegexHighlighting();
	}
};

onUnmounted(() => {
	editorRef.value?.deltaDecorations(decorations.value, []);
});

watch(props.regexes, () => {
	applyRegexHighlighting();
}, { deep: true });

watch(() => props.text, (newValue) => {
	internalText.value = newValue;
	console.log(internalText.value);
	if (editorRef.value && editorRef.value.getValue() !== newValue) {
		editorRef.value.setValue(newValue);
	}
});

function applyRegexHighlighting() {
	if (!editorRef.value) return;
	const model = editorRef.value.getModel() ?? undefined;
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
	decorations.value = editorRef.value?.deltaDecorations(decorations.value, newDecorations) ?? decorations.value;
}
</script>