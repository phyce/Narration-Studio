// Mirror the Go-side python.Manifest, pack.AvailablePack and pack.InstalledPack.
export interface ManifestVoice {
	id: string;
	name: string;
	gender: string;
}

export interface ManifestModel {
	id: string;
	name: string;
	voices: ManifestVoice[] | null;
}

export interface PythonManifest {
	schemaVersion: number;
	protocol: number;
	id: string;
	name: string;
	version: string;
	variant: string;
	variantDescription?: string;
	description?: string;
	type: 'local' | 'api';
	tags: string[] | null;
	os: string;
	arch: string;
	approxInstallSizeMB?: number;
	runtime?: {prepare?: {downloadSizeMB: number; source: string} | null};
	models: ManifestModel[] | null;
}

export interface AvailablePack {
	manifest: PythonManifest;
	bundleUrl: string;
	bundleSize: number;
	sha256?: string;
	notes?: string;
	publishedAt?: string;
	local: boolean;
	compatible: boolean;
	paused: boolean;
}

export interface InstalledPack {
	manifest: PythonManifest;
	compatible: boolean;
}

export interface EnginePackListing {
	packs: AvailablePack[];
	installed: InstalledPack[];
}
