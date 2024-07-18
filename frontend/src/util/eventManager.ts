// import { EventsOff, EventsOn, EventsEmit } from '@wailsapp/runtime';
import { EventsOff, EventsOn, EventsEmit } from '../../wailsjs/runtime';

class EventManager {
	private static instance: EventManager;
	private subscriptions: { [key: string]: (() => void)[] } = {};

	private constructor() {}

	public static getInstance(): EventManager {
		if (!EventManager.instance) {
			EventManager.instance = new EventManager();
		}
		return EventManager.instance;
	}

	subscribe(eventName: string, callback: (data?: any) => void): void {
		const unsubscribe = EventsOn(eventName, callback);
		if (!this.subscriptions[eventName]) {
			this.subscriptions[eventName] = [];
		}
		this.subscriptions[eventName].push(unsubscribe);
	}

	unsubscribe(eventName: string): void {
		if (this.subscriptions[eventName]) {
			this.subscriptions[eventName].forEach((unsubscribe) => unsubscribe());
			delete this.subscriptions[eventName];
		}
	}

	emit(eventName: string, data?: any): void {
		EventsEmit(eventName, data);
	}

	dispose(): void {
		Object.keys(this.subscriptions).forEach(this.unsubscribe.bind(this));
	}
}

export const eventManager = EventManager.getInstance();