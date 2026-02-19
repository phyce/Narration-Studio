export namespace config {
	
	export class Gemini {
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new Gemini(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	    }
	}
	export class Google {
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new Google(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	    }
	}
	export class ElevenLabs {
	    apiKey: string;
	    outputType: string;
	
	    static createFrom(source: any = {}) {
	        return new ElevenLabs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.outputType = source["outputType"];
	    }
	}
	export class OpenAI {
	    apiKey: string;
	    outputType: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenAI(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.outputType = source["outputType"];
	    }
	}
	export class Api {
	    openAI: OpenAI;
	    elevenLabs: ElevenLabs;
	    google: Google;
	    gemini: Gemini;
	
	    static createFrom(source: any = {}) {
	        return new Api(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.openAI = this.convertValues(source["openAI"], OpenAI);
	        this.elevenLabs = this.convertValues(source["elevenLabs"], ElevenLabs);
	        this.google = this.convertValues(source["google"], Google);
	        this.gemini = this.convertValues(source["gemini"], Gemini);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AudioCacheSettings {
	    enabled: boolean;
	    location: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioCacheSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.location = source["location"];
	    }
	}
	export class AuthSettings {
	    key: string;
	    adminKey: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.adminKey = source["adminKey"];
	    }
	}
	export class Info {
	    name: string;
	    version: string;
	    website: string;
	    os: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.website = source["website"];
	        this.os = source["os"];
	    }
	}
	export class MsSapi4 {
	    location: string;
	    pitch: number;
	    speed: number;
	
	    static createFrom(source: any = {}) {
	        return new MsSapi4(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.location = source["location"];
	        this.pitch = source["pitch"];
	        this.speed = source["speed"];
	    }
	}
	export class Piper {
	    useExecutable: boolean;
	    location: string;
	    modelsDirectory: string;
	    useGPU: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Piper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.useExecutable = source["useExecutable"];
	        this.location = source["location"];
	        this.modelsDirectory = source["modelsDirectory"];
	        this.useGPU = source["useGPU"];
	    }
	}
	export class Local {
	    piper: Piper;
	    msSapi4: MsSapi4;
	
	    static createFrom(source: any = {}) {
	        return new Local(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.piper = this.convertValues(source["piper"], Piper);
	        this.msSapi4 = this.convertValues(source["msSapi4"], MsSapi4);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Engine {
	    local: Local;
	    api: Api;
	
	    static createFrom(source: any = {}) {
	        return new Engine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.local = this.convertValues(source["local"], Local);
	        this.api = this.convertValues(source["api"], Api);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelInstances {
	    instances: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelInstances(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instances = source["instances"];
	    }
	}
	export class ServerSettingsEngines {
	    piper?: Record<string, ModelInstances>;
	    openai?: Record<string, ModelInstances>;
	    elevenlabs?: Record<string, ModelInstances>;
	    mssapi4?: Record<string, ModelInstances>;
	    google?: Record<string, ModelInstances>;
	    gemini?: Record<string, ModelInstances>;
	
	    static createFrom(source: any = {}) {
	        return new ServerSettingsEngines(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.piper = this.convertValues(source["piper"], ModelInstances, true);
	        this.openai = this.convertValues(source["openai"], ModelInstances, true);
	        this.elevenlabs = this.convertValues(source["elevenlabs"], ModelInstances, true);
	        this.mssapi4 = this.convertValues(source["mssapi4"], ModelInstances, true);
	        this.google = this.convertValues(source["google"], ModelInstances, true);
	        this.gemini = this.convertValues(source["gemini"], ModelInstances, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServerSettings {
	    auth?: AuthSettings;
	    engines: ServerSettingsEngines;
	
	    static createFrom(source: any = {}) {
	        return new ServerSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auth = this.convertValues(source["auth"], AuthSettings);
	        this.engines = this.convertValues(source["engines"], ServerSettingsEngines);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    outputType: number;
	    outputPath: string;
	    debug: boolean;
	    audioCache?: AudioCacheSettings;
	    server?: ServerSettings;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputType = source["outputType"];
	        this.outputPath = source["outputPath"];
	        this.debug = source["debug"];
	        this.audioCache = this.convertValues(source["audioCache"], AudioCacheSettings);
	        this.server = this.convertValues(source["server"], ServerSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Base {
	    settings: Settings;
	    engine: Engine;
	    modelToggles: Record<string, boolean>;
	    info: Info;
	
	    static createFrom(source: any = {}) {
	        return new Base(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.engine = this.convertValues(source["engine"], Engine);
	        this.modelToggles = source["modelToggles"];
	        this.info = this.convertValues(source["info"], Info);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	
	
	
	
	

}

