export enum OutputType {
    CombinedFile = 0,
    SplitFiles = 1,
    Both = 2,
}

export const OutputTypeNames = {
  [OutputType.CombinedFile]: 'Combined File',
  [OutputType.SplitFiles]: 'Split Files',
  [OutputType.Both]: 'Split & Combined Files',
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