import type {AvailablePack, InstalledPack, PythonManifest} from '../components/interfaces/enginePack';

export function compareVersions(left: string, right: string): number {
	const leftParts = left.split('.').map(Number);
	const rightParts = right.split('.').map(Number);
	for (let index = 0; index < Math.max(leftParts.length, rightParts.length); index++) {
		const difference = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
		if (difference !== 0) {
			return difference;
		}
	}
	return 0;
}

export function isSameBuild(left: PythonManifest, right: PythonManifest): boolean {
	return left.id === right.id && left.version === right.version && left.variant === right.variant;
}

export function newestFirst(packs: AvailablePack[]): AvailablePack[] {
	return [...packs].sort((left, right) =>
		compareVersions(right.manifest.version, left.manifest.version)
		|| left.manifest.variant.localeCompare(right.manifest.variant)
		|| Number(left.local) - Number(right.local));
}

export function findUpdate(builds: AvailablePack[], installed: InstalledPack | undefined): AvailablePack | undefined {
	if (!installed) {
		return undefined;
	}
	return newestFirst(builds).find(build =>
		build.compatible
		&& build.manifest.variant === installed.manifest.variant
		&& compareVersions(build.manifest.version, installed.manifest.version) > 0);
}
