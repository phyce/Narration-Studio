export enum OutputType {
	CombinedFile = 0,
	SplitFiles = 1,
	Both = 2,
}

export const OutputTypeNames = {
	[OutputType.CombinedFile]: 'Combined File',
	[OutputType.SplitFiles]: 'Individual Files',
	[OutputType.Both]: 'Both Individual and Combined',
};

export const OutputTypeOptions = Object.keys(OutputType)
	.filter((key) => !isNaN(Number(key)))
	.map((key) => {
		const value = Number(key) as OutputType;
		const label = OutputTypeNames[value];
		return {
			value,
			name: label,
			label,
		};
	});