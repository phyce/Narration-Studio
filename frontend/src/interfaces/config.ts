export interface FieldCondition {
	field: string;
	value: any;
}

export interface ConfigFieldMetadata {
	label: string;
	type: 'text' | 'password' | 'number' | 'checkbox' | 'path' | 'dropdown' | 'object';
	pathType?: 'file' | 'directory';
	options?: Array<{ value: any; label: string }>;
	description?: string;
	placeholder?: string;
	min?: number;
	max?: number;
	required?: boolean;
	dynamic?: boolean;
	valueType?: string;
	hidden?: boolean;
	hideWhen?: FieldCondition | FieldCondition[];
	disableWhen?: FieldCondition | FieldCondition[];
}

export interface ConfigField {
	path: string;
	value: any;
	metadata?: ConfigFieldMetadata;
}

export interface ConfigSchema {
	fields: ConfigField[];
}

export interface ConfigSchemaResponse {
	success: boolean;
	schema: ConfigSchema;
}
